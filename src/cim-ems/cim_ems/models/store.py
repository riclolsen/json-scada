"""Model revisions: immutable imports, explicit activation, rollback."""
from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

import pypowsybl
from pymongo.database import Database

from .. import CGMES_VERSION
from ..mongo import MODELS, artifacts
from .importer import IMPORT_PARAMETERS, ValidationReport, sha256_file

PROFILES = ["EQ", "SSH", "TP", "SV"]


class ModelError(Exception):
    pass


def register_import(
    db: Database,
    package: str | Path,
    model_id: str,
    report: ValidationReport,
    user: str = "cli",
    boundary_ref: str | None = None,
) -> dict:
    """Store the original package and the validation outcome as a new revision.

    A model with validation errors is stored as `rejected` so the report stays
    reviewable, but it can never be activated.
    """
    package = Path(package)
    sha = sha256_file(package)
    fs = artifacts(db)
    with open(package, "rb") as f:
        source_id = fs.put(f, filename=package.name, kind="cgmes-package", modelId=model_id, sha256=sha)

    last = db[MODELS].find_one({"modelId": model_id}, sort=[("revision", -1)])
    revision = (last["revision"] + 1) if last else 1
    doc = {
        "modelId": model_id,
        "revision": revision,
        "status": "validated" if report.ok else "rejected",
        "cgmesVersion": CGMES_VERSION,
        "profiles": PROFILES,
        "boundaryRef": boundary_ref,
        "sourceArtifactId": source_id,
        "sourceSha256": sha,
        "importedBy": user,
        "importedAt": datetime.now(timezone.utc),
        "engine": {"pypowsybl": _pypowsybl_version(), "importParameters": IMPORT_PARAMETERS},
        "validation": report.as_dict(),
        "counts": report.counts,
    }
    db[MODELS].insert_one(doc)
    return doc


def _pypowsybl_version() -> str:
    from importlib.metadata import version

    return version("pypowsybl")


def activate(db: Database, model_id: str, revision: int) -> dict:
    """Make `revision` the single active revision; the previous one becomes `retired`."""
    target = db[MODELS].find_one({"modelId": model_id, "revision": revision})
    if target is None:
        raise ModelError(f"{model_id} revision {revision} not found")
    if target["status"] == "rejected":
        raise ModelError(f"{model_id} revision {revision} was rejected by validation")
    if target["status"] == "active":
        return target
    db[MODELS].update_many({"modelId": model_id, "status": "active"}, {"$set": {"status": "retired"}})
    db[MODELS].update_one(
        {"_id": target["_id"]}, {"$set": {"status": "active", "activatedAt": datetime.now(timezone.utc)}}
    )
    return db[MODELS].find_one({"_id": target["_id"]})


def rollback(db: Database, model_id: str) -> dict:
    """Activate the most recent previously-activated revision before the current one."""
    active = db[MODELS].find_one({"modelId": model_id, "status": "active"})
    if active is None:
        raise ModelError(f"{model_id} has no active revision")
    prev = db[MODELS].find_one(
        {"modelId": model_id, "status": "retired", "revision": {"$lt": active["revision"]}},
        sort=[("revision", -1)],
    )
    if prev is None:
        raise ModelError(f"{model_id} has no earlier revision to roll back to")
    return activate(db, model_id, prev["revision"])


def get_active(db: Database, model_id: str) -> dict | None:
    return db[MODELS].find_one({"modelId": model_id, "status": "active"})


def load_network(db: Database, doc: dict):
    """Rebuild the pypowsybl network of a stored revision from its original package."""
    import tempfile

    from .importer import import_cgmes

    with tempfile.TemporaryDirectory() as tmp:
        f = Path(tmp) / "model.zip"
        f.write_bytes(artifacts(db).get(doc["sourceArtifactId"]).read())
        if sha256_file(f) != doc["sourceSha256"]:
            raise ModelError("stored package does not match its recorded SHA-256")
        return import_cgmes(f)
