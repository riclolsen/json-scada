"""Versioned binding sets in `cimMeasurementBindings`."""
from __future__ import annotations

from dataclasses import asdict
from datetime import datetime, timezone

from pymongo.database import Database

from ..mongo import BINDINGS
from .model import Binding


def save_revision(db: Database, model_id: str, bindings: list[Binding], user: str = "cli") -> int:
    """Store a complete binding set as the next revision. Returns the new `bindingRevision`."""
    last = db[BINDINGS].find_one({"modelId": model_id}, sort=[("bindingRevision", -1)])
    rev = (last["bindingRevision"] + 1) if last else 1
    now = datetime.now(timezone.utc)
    docs = []
    for b in bindings:
        b.validate()
        d = asdict(b)
        d.update({"modelId": model_id, "bindingRevision": rev, "createdAt": now, "createdBy": user,
                  "cimMeasurementMrid": d.pop("element_id")})
        docs.append(d)
    if docs:
        db[BINDINGS].insert_many(docs)
    return rev


def latest_revision(db: Database, model_id: str) -> int | None:
    last = db[BINDINGS].find_one({"modelId": model_id}, sort=[("bindingRevision", -1)])
    return last["bindingRevision"] if last else None


def load_revision(db: Database, model_id: str, revision: int, approved_only: bool = True) -> list[Binding]:
    out = []
    for d in db[BINDINGS].find({"modelId": model_id, "bindingRevision": revision}):
        d["element_id"] = d.pop("cimMeasurementMrid")
        b = Binding(**{k: d[k] for k in Binding.__dataclass_fields__ if k in d})
        if b.approved or not approved_only:
            out.append(b)
    return out
