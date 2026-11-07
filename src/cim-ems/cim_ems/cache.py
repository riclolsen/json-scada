"""In-memory copy of the processed `realtimeData` values of the bound points.

The change stream is opened before the initial load, so no update falls between the two. Any stream error, an
invalidate, or `request_resync()` triggers a full reload and a new stream. Values are the processed ones
(`value`, flags and time tags after kconv and alarm processing), never `sourceDataUpdate`, so conversions are
not applied twice.
"""
from __future__ import annotations

import logging
import threading
import time
from datetime import datetime, timezone

from pymongo.database import Database
from pymongo.errors import PyMongoError

from .snapshot import PointState

log = logging.getLogger("CIM-EMS.cache")

FIELDS = {"tag": 1, "value": 1, "invalid": 1, "transient": 1, "substituted": 1, "frozen": 1,
          "timeTag": 1, "timeTagAtSource": 1, "timeTagAtSourceOk": 1}


def _aware(d):
    return d.replace(tzinfo=timezone.utc) if isinstance(d, datetime) and d.tzinfo is None else d


def to_state(doc: dict) -> PointState:
    value = doc.get("value")
    return PointState(
        value=float(value) if isinstance(value, (int, float)) and not isinstance(value, bool) else
        (float(value) if isinstance(value, bool) else None),
        invalid=bool(doc.get("invalid", False)),
        transient=bool(doc.get("transient", False)),
        substituted=bool(doc.get("substituted", False)),
        frozen=bool(doc.get("frozen", False)),
        time_tag=_aware(doc.get("timeTag")),
        time_tag_at_source=_aware(doc.get("timeTagAtSource")),
        time_tag_at_source_ok=bool(doc.get("timeTagAtSourceOk", False)),
    )


def _key(s: PointState) -> tuple:
    return (s.value, s.invalid, s.transient, s.substituted, s.frozen)


class PointCache:
    def __init__(self, db: Database, tags: list[str], trigger_tags: set[str] | None = None,
                 resync_seconds: float = 600.0):
        self.col = db["realtimeData"]
        self.tags = sorted(set(tags))
        self.trigger_tags = set(trigger_tags or ())
        self.resync_seconds = resync_seconds
        self.trigger = threading.Event()      # set when a trigger tag (switch/tap) changes
        self.ready = threading.Event()        # set after the first full load
        self._stop = threading.Event()
        self._resync = threading.Event()
        self._lock = threading.Lock()
        self._points: dict[str, PointState] = {}
        self._thread: threading.Thread | None = None
        self.stats = {"events": 0, "full_loads": 0, "stream_errors": 0}

    # --- public -----------------------------------------------------------
    def points(self) -> dict[str, PointState]:
        with self._lock:
            return dict(self._points)

    def request_resync(self) -> None:
        self._resync.set()

    def start(self) -> None:
        self._thread = threading.Thread(target=self._run, name="cim-ems-cache", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=5)

    # --- internals --------------------------------------------------------
    def full_load(self) -> None:
        fresh = {d["tag"]: to_state(d) for d in self.col.find({"tag": {"$in": self.tags}}, FIELDS)}
        with self._lock:
            changed = {t for t in self.trigger_tags if _key(fresh[t]) != _key(self._points[t])} \
                if self._points and all(t in fresh and t in self._points for t in self.trigger_tags) else self.trigger_tags
            self._points = fresh
        self.stats["full_loads"] += 1
        if changed:
            self.trigger.set()
        log.info("full load: %d of %d points found", len(fresh), len(self.tags))

    def apply(self, doc: dict) -> None:
        tag = doc.get("tag")
        if tag is None:
            return
        new = to_state(doc)
        with self._lock:
            old = self._points.get(tag)
            self._points[tag] = new
        self.stats["events"] += 1
        if tag in self.trigger_tags and (old is None or _key(old) != _key(new)):
            self.trigger.set()

    def _run(self) -> None:
        pipeline = [{"$match": {"operationType": {"$in": ["insert", "update", "replace"]},
                                "fullDocument.tag": {"$in": self.tags}}}]
        last_full = time.monotonic()
        while not self._stop.is_set():
            try:
                with self.col.watch(pipeline, full_document="updateLookup", max_await_time_ms=500) as stream:
                    self.full_load()                      # after the stream is open: no gap
                    last_full = time.monotonic()
                    self._resync.clear()
                    self.ready.set()
                    while not self._stop.is_set() and not self._resync.is_set():
                        ev = stream.try_next()
                        if ev is not None and ev.get("fullDocument"):
                            self.apply(ev["fullDocument"])
                        elif ev is None and time.monotonic() - last_full > self.resync_seconds:
                            break                         # periodic safety resync
                    if self._resync.is_set():
                        log.info("resync requested")
            except PyMongoError as e:
                self.stats["stream_errors"] += 1
                log.warning("change stream error, resyncing: %s", e)
                self._stop.wait(1.0)
