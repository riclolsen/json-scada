"""Result point registry and publication through `sourceDataUpdate`.

The data processor owns alarms, SOE and the historian; this module never writes them. Result points are
`origin: supervised` points of the CIM-EMS connection. Their address is
(protocolSourceCommonAddress = study kind, protocolSourceObjectAddress = "<mRID>|<side>|<quantity>").
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone

from pymongo import UpdateOne
from pymongo.database import Database

from .redundancy import Redundancy
from .results import ResultValue

log = logging.getLogger("CIM-EMS.publish")

KEY_MULTIPLIER = 1_000_000   # point keys of a connection: connection * multiplier + n


def tag_name(connection: str, rv: ResultValue, ambiguous: bool) -> str:
    base = rv.name if rv.name and not ambiguous else rv.element_id
    tag = f"{connection};{rv.kind};{base};{rv.quantity}"
    return tag + (f";{rv.side}" if rv.side else "")


def point_document(key: float, tag: str, rv: ResultValue, connection_number: int, result_max_age: float,
                   now: datetime) -> dict:
    """realtimeData template, same field set as the other auto-creating drivers."""
    d = float
    return {
        "_id": d(key), "protocolSourceASDU": "analog", "protocolSourceCommonAddress": rv.kind,
        "protocolSourceConnectionNumber": d(connection_number), "protocolSourceObjectAddress": rv.address,
        "protocolSourceCommandDuration": d(0), "protocolSourceCommandUseSBO": False,
        "alarmState": d(-1.0), "alarmRange": d(0.0),
        "description": f"{rv.kind} {rv.group1} {rv.group2} {rv.name or rv.element_id} {rv.quantity}"
                       + (f" side {rv.side}" if rv.side else ""),
        "ungroupedDescription": f"{rv.name or rv.element_id} {rv.quantity}" + (f" side {rv.side}" if rv.side else ""),
        "group1": rv.group1, "group2": rv.group2, "group3": rv.kind,
        "stateTextFalse": "", "stateTextTrue": "", "eventTextFalse": "", "eventTextTrue": "",
        "origin": "supervised", "tag": tag, "type": "analog", "value": d(0.0), "valueString": "0",
        "alarmDisabled": False, "alerted": False, "alarmed": False, "alertState": "", "annotation": "",
        "commandBlocked": False, "commandOfSupervised": d(0.0),
        "commissioningRemarks": "Auto created by CIM-EMS", "formula": d(0.0),
        "frozen": False, "frozenDetectTimeout": d(0.0),
        "hiLimit": d(1.7976931348623157e308), "hihiLimit": d(1.7976931348623157e308),
        "hihihiLimit": d(1.7976931348623157e308), "historianDeadBand": d(0.0), "historianPeriod": d(0.0),
        "hysteresis": d(0.0), "invalid": True, "invalidDetectTimeout": d(float(result_max_age)),
        "isEvent": False, "kconv1": d(1.0), "kconv2": d(0.0), "location": None,
        "loLimit": d(-1.7976931348623157e308), "loloLimit": d(-1.7976931348623157e308),
        "lololoLimit": d(-1.7976931348623157e308), "notes": "", "overflow": False, "parcels": None,
        "priority": d(0.0), "protocolDestinations": None, "sourceDataUpdate": None, "substituted": False,
        "supervisedOfCommand": d(0.0), "timeTag": None, "timeTagAlarm": None, "timeTagAtSource": None,
        "timeTagAtSourceOk": False, "transient": False, "unit": rv.unit, "updatesCnt": d(0.0),
        "valueDefault": d(0.0), "zeroDeadband": d(0.0),
    }


class ResultPublisher:
    def __init__(self, db: Database, redundancy: Redundancy, connection_number: int, connection_name: str,
                 result_max_age: float = 180.0, auto_create: bool = True):
        self.col = db["realtimeData"]
        self.red = redundancy
        self.number = connection_number
        self.name = connection_name
        self.result_max_age = result_max_age
        self.auto_create = auto_create
        self.keys: dict[tuple[str, str], tuple[float, str]] = {}   # (kind, address) -> (key, tag)
        self.last_run: dict[str, int] = {}                         # kind -> last published runId
        self._load_existing()

    def _load_existing(self) -> None:
        for d in self.col.find({"protocolSourceConnectionNumber": self.number, "origin": "supervised"},
                               {"protocolSourceCommonAddress": 1, "protocolSourceObjectAddress": 1, "tag": 1}):
            self.keys[(d["protocolSourceCommonAddress"], d["protocolSourceObjectAddress"])] = (float(d["_id"]), d["tag"])

    def _next_key(self) -> float:
        base = self.number * KEY_MULTIPLIER
        last = self.col.find_one({"_id": {"$gt": base, "$lt": base + KEY_MULTIPLIER}}, {"_id": 1}, sort=[("_id", -1)])
        return float(last["_id"]) if last else float(base)

    def ensure_points(self, values: list[ResultValue]) -> int:
        """Create the missing result points. Returns how many were created."""
        missing = [v for v in values if (v.kind, v.address) not in self.keys]
        if not missing or not self.auto_create:
            return 0
        counts: dict[tuple, int] = {}
        for v in values:
            counts[(v.kind, v.name, v.quantity, v.side)] = counts.get((v.kind, v.name, v.quantity, v.side), 0) + 1
        key = self._next_key()
        now = datetime.now(timezone.utc)
        docs = []
        for v in missing:
            key += 1
            tag = tag_name(self.name, v, counts[(v.kind, v.name, v.quantity, v.side)] > 1)
            docs.append(point_document(key, tag, v, self.number, self.result_max_age, now))
            self.keys[(v.kind, v.address)] = (key, tag)
        self.col.insert_many(docs, ordered=False)
        log.info("created %d result points", len(docs))
        return len(docs)

    def publish(self, run_id: int, snapshot_time: datetime, values: list[ResultValue], originator_run: str = "") -> int:
        """Write all values of one study run. Returns the number of points written (0 when refused)."""
        if not values:
            return 0
        kind = values[0].kind
        if run_id <= self.last_run.get(kind, 0):
            log.warning("run %d is not newer than published run %d, refused", run_id, self.last_run[kind])
            return 0
        if not self.red.still_active():
            log.warning("node lost the active role, run %d discarded", run_id)
            return 0
        self.ensure_points(values)
        now = datetime.now(timezone.utc)
        ops = []
        for v in values:
            ref = self.keys.get((v.kind, v.address))
            if ref is None:
                continue
            valid = v.value is not None
            ops.append(UpdateOne({"_id": ref[0]}, {"$set": {"sourceDataUpdate": {
                "valueAtSource": float(v.value if valid else 0.0),
                "valueStringAtSource": f"{v.value:g}" if valid else "0",
                "invalidAtSource": not valid,
                "transientAtSource": False,
                "timeTagAtSource": snapshot_time,
                "timeTagAtSourceOk": True,
                "timeTag": now,
                "originator": f"CIM-EMS|{self.number}|{originator_run or run_id}",
                "asduAtSource": "analog",
            }}}))
        if ops:
            self.col.bulk_write(ops, ordered=False)
        self.last_run[kind] = run_id
        return len(ops)

    def invalidate(self, kind: str) -> int:
        """Mark all points of a study kind invalid at source (failed or expired results)."""
        if not self.red.still_active():
            return 0
        now = datetime.now(timezone.utc)
        ids = [k for (kk, _), (k, _) in self.keys.items() if kk == kind]
        if not ids:
            return 0
        self.col.update_many({"_id": {"$in": ids}, "sourceDataUpdate": {"$ne": None}},
                             {"$set": {"sourceDataUpdate.invalidAtSource": True, "sourceDataUpdate.timeTag": now}})
        # never published (new or seeded point): a null subdocument cannot take dotted fields
        self.col.update_many({"_id": {"$in": ids}, "sourceDataUpdate": None}, {"$set": {"sourceDataUpdate": {
            "valueAtSource": 0.0, "valueStringAtSource": "0", "invalidAtSource": True, "transientAtSource": False,
            "timeTagAtSource": now, "timeTagAtSourceOk": False, "timeTag": now,
            "originator": f"CIM-EMS|{self.number}|invalidate", "asduAtSource": "analog"}}})
        return len(ids)
