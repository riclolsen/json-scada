"""Snapshot policy: turn processed point states into a consistent operating state.

Rules (CIM_EMS_PLAN.md section 8):
- A switch closed/open only if the point is valid, not transient and has a 0/closed value.
- Transient switch: keep the last valid state, flag topologyUncertain.
- Invalid switch: keep the last valid state while younger than `max_hold_seconds`, else reject the snapshot.
- Never assume a switch is closed. Without any valid history the model (SSH) state stays and the run is flagged.
- Analogs are dropped when invalid, older than their limit, or farther than `max_skew_seconds` from the median time.
- Switch values are not aged: a stable breaker legitimately has an old timestamp.
"""
from __future__ import annotations

import statistics
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone

from .bindings.model import Binding


@dataclass
class PointState:
    value: float | None
    invalid: bool = False
    transient: bool = False
    substituted: bool = False
    frozen: bool = False
    time_tag: datetime | None = None
    time_tag_at_source: datetime | None = None
    time_tag_at_source_ok: bool = False

    @property
    def time(self) -> datetime | None:
        return self.time_tag_at_source if self.time_tag_at_source_ok and self.time_tag_at_source else self.time_tag


@dataclass
class SnapshotPolicy:
    max_hold_seconds: float = 300.0
    max_skew_seconds: float = 10.0


@dataclass
class Measurement:
    element_id: str
    quantity: str
    side: int
    value: float
    sigma: float
    substituted: bool = False
    tag: str = ""                 # JSON-SCADA tag of the bound point


@dataclass
class Snapshot:
    time: datetime
    switches: dict[str, bool] = field(default_factory=dict)   # id -> open
    taps: dict[str, int] = field(default_factory=dict)
    measurements: list[Measurement] = field(default_factory=list)
    flags: set[str] = field(default_factory=set)
    reasons: list[str] = field(default_factory=list)
    rejected: bool = False


def _aware(d: datetime | None) -> datetime | None:
    return d.replace(tzinfo=timezone.utc) if d is not None and d.tzinfo is None else d


class SnapshotBuilder:
    def __init__(self, bindings: list[Binding], policy: SnapshotPolicy | None = None,
                 tap_ranges: dict[str, tuple[int, int]] | None = None):
        self.bindings = [b for b in bindings if b.approved]
        self.policy = policy or SnapshotPolicy()
        self.tap_ranges = tap_ranges or {}
        # element id -> (value, time of last valid sample)
        self.last_switch: dict[str, tuple[bool, datetime]] = {}
        self.last_tap: dict[str, tuple[int, datetime]] = {}

    def build(self, points: dict[str, PointState], now: datetime) -> Snapshot:
        now = _aware(now)
        snap = Snapshot(time=now)
        hold = timedelta(seconds=self.policy.max_hold_seconds)
        for b in self.bindings:
            if b.kind == "switch":
                self._discrete(snap, b, points.get(b.tag), now, hold, self.last_switch, snap.switches,
                               lambda v, b=b: self._switch_value(v, b))
            elif b.kind == "tap":
                self._discrete(snap, b, points.get(b.tag), now, hold, self.last_tap, snap.taps,
                               lambda v, b=b: self._tap_value(v, b))
        self._analogs(snap, points, now)
        return snap

    @staticmethod
    def _switch_value(v: float | None, b: Binding) -> bool | None:
        if v is None or v not in (0, 1, 0.0, 1.0):
            return None
        return not (v == b.closed_value)   # returns "open"

    def _tap_value(self, v: float | None, b: Binding) -> int | None:
        if v is None or float(v) != int(v):
            return None
        rng = self.tap_ranges.get(b.element_id)
        if rng and not (rng[0] <= int(v) <= rng[1]):
            return None
        return int(v)

    def _discrete(self, snap, b, p, now, hold, last, out, convert):
        value = None if p is None or p.invalid else convert(p.value)
        if p is not None and p.transient:
            snap.flags.add("topologyUncertain")
            snap.reasons.append(f"{b.tag}: transient, last valid state kept")
            if b.element_id in last:
                out[b.element_id] = last[b.element_id][0]
            return
        if value is not None:
            last[b.element_id] = (value, now)
            out[b.element_id] = value
            if p.substituted:
                snap.flags.add("substituted")
            return
        # invalid, unreadable or missing
        snap.flags.add("topologyUncertain")
        if b.element_id in last:
            state, t = last[b.element_id]
            if now - t <= hold:
                out[b.element_id] = state
                snap.reasons.append(f"{b.tag}: invalid, held last valid state")
            else:
                snap.rejected = True
                snap.reasons.append(f"{b.tag}: invalid for more than {hold.total_seconds():.0f} s")
        else:
            snap.reasons.append(f"{b.tag}: invalid with no history, model state kept")

    def _analogs(self, snap: Snapshot, points: dict[str, PointState], now: datetime) -> None:
        cand = []
        for b in self.bindings:
            if b.kind != "analog":
                continue
            p = points.get(b.tag)
            t = _aware(p.time) if p else None
            if p is None or p.invalid or p.frozen or p.value is None or t is None:
                snap.reasons.append(f"{b.tag}: analog dropped (invalid or missing)")
                continue
            if (now - t).total_seconds() > b.max_age_seconds:
                snap.reasons.append(f"{b.tag}: analog dropped (older than {b.max_age_seconds:.0f} s)")
                continue
            cand.append((b, p, t))
        if cand:
            ref = statistics.median_high(t.timestamp() for _, _, t in cand)   # with two samples, trust the newer
            for b, p, t in cand:
                if abs(t.timestamp() - ref) > self.policy.max_skew_seconds:
                    snap.reasons.append(f"{b.tag}: analog dropped (time skew)")
                    continue
                snap.measurements.append(Measurement(b.element_id, b.quantity, b.side, p.value * b.sign,
                                                     b.sigma, p.substituted, b.tag))
                if p.substituted:
                    snap.flags.add("substituted")
