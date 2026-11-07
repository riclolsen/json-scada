"""Active-node arbitration, as in docs/DEVELOPER_GUIDE.md (protocolDriverInstances keep-alive)."""
from __future__ import annotations

import os
from datetime import datetime, timedelta, timezone

from pymongo.database import Database

KEEPALIVE_TIMEOUT = timedelta(seconds=float(os.environ.get("CIM_EMS_KEEPALIVE_TIMEOUT", 30)))
DRIVER_NAME = "CIM-EMS"


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


def _aware(d: datetime | None) -> datetime | None:
    if d is not None and d.tzinfo is None:
        return d.replace(tzinfo=timezone.utc)
    return d


class Redundancy:
    """Call `tick()` about once per second. `is_active` tells whether this node may publish."""

    def __init__(self, db: Database, node_name: str, instance: int, timeout: timedelta = KEEPALIVE_TIMEOUT):
        self.col = db["protocolDriverInstances"]
        self.node = node_name
        self.instance = instance
        self.timeout = timeout
        self.is_active = False

    def _key(self) -> dict:
        return {"protocolDriver": DRIVER_NAME, "protocolDriverInstanceNumber": self.instance}

    def instance_doc(self) -> dict | None:
        return self.col.find_one(self._key())

    def tick(self, now: datetime | None = None) -> bool:
        now = now or utcnow()
        doc = self.instance_doc()
        if doc is None or not doc.get("enabled", False) or self.node not in doc.get("nodeNames", []):
            self.is_active = False
            return False
        # Atomic take-over: succeeds only if still free, ours, or stale.
        stale = now - self.timeout
        res = self.col.update_one(
            {
                **self._key(),
                "$or": [
                    {"activeNodeName": {"$in": [None, "", self.node]}},
                    {"activeNodeKeepAliveTimeTag": {"$lt": stale}},
                    {"activeNodeKeepAliveTimeTag": None},
                ],
            },
            {"$set": {"activeNodeName": self.node, "activeNodeKeepAliveTimeTag": now}},
        )
        self.is_active = res.modified_count == 1 or res.matched_count == 1
        return self.is_active

    def still_active(self) -> bool:
        """Check immediately before publishing: the node must still own the instance."""
        doc = self.instance_doc()
        return bool(doc and doc.get("activeNodeName") == self.node and doc.get("enabled", False))
