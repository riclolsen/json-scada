"""Study requests queued by the Admin UI backend in `cimStudyRequests`, executed on the active node.

Request document: {type, params, status, createdAt, createdBy}. The driver claims one atomically
(`pending` -> `running`), executes it and sets `done` with a `result` or `failed` with an `error`.

Types: `import` (uploaded package in the artifact bucket), `activate`, `rollback`, `proposeBindings`,
`listElements` (element catalog for binding review), `saveBindings` (binding list in the artifact bucket) and
`rerun`. The backend checks the user rights; the
driver re-checks the contents (validation, own-point refusal) because it owns the model and binding rules.
"""
from __future__ import annotations

import json
import logging
import re
import tempfile
from dataclasses import asdict, dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path

from bson import ObjectId
from pymongo import ReturnDocument
from pymongo.database import Database

from .bindings import store as bstore
from .bindings.model import Binding
from .mongo import ARTIFACT_BUCKET, MODELS, STUDY_REQUESTS, artifacts
from .models import store as mstore

log = logging.getLogger("CIM-EMS")

STALE_SECONDS = 3600          # a request left `running` this long belonged to a node that died
CATALOG_KEEP = 5              # element catalogs kept per model (each listElements request writes one)
LIST_LIMIT = 500              # unmatched/ambiguous ids kept in a request result


@dataclass
class Effects:
    """What the main loop must do after the processed requests."""
    rerun: set[int] = field(default_factory=set)          # connection numbers to run now
    reload_models: set[str] = field(default_factory=set)  # model ids whose cycles must be rebuilt


class RequestError(Exception):
    pass


def _now() -> datetime:
    return datetime.now(timezone.utc)


def _oid(v) -> ObjectId:
    if isinstance(v, ObjectId):
        return v
    if isinstance(v, dict) and "$oid" in v:
        v = v["$oid"]
    if not ObjectId.is_valid(str(v)):
        raise RequestError(f"invalid artifact id {v!r}")
    return ObjectId(str(v))


def _model_id(params: dict) -> str:
    mid = str(params.get("modelId", ""))
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,63}", mid):
        raise RequestError("invalid modelId")
    return mid


def _put_json(db: Database, obj, filename: str, **meta) -> ObjectId:
    return artifacts(db).put(json.dumps(obj, default=str).encode(), filename=filename, **meta)


def do_import(db: Database, req: dict, fx: Effects) -> dict:
    from .models.importer import import_cgmes, validate

    p = req["params"]
    mid = _model_id(p)
    upload = _oid(p.get("artifactId"))
    fs = artifacts(db)
    name = re.sub(r"[^A-Za-z0-9._-]", "_", Path(str(p.get("fileName") or "model.zip")).name) or "model.zip"
    with tempfile.TemporaryDirectory() as tmp:
        f = Path(tmp) / name
        f.write_bytes(fs.get(upload).read())
        try:
            report = validate(import_cgmes(f))
        except Exception as e:     # not a CGMES package pypowsybl can read: nothing is registered
            raise RequestError(f"import failed: {type(e).__name__}: {str(e)[:300]}") from e
        doc = mstore.register_import(db, f, mid, report, user=req.get("createdBy") or "admin-ui")
    fs.delete(upload)              # register_import keeps its own copy as the revision source
    v = doc["validation"]
    return {"modelId": mid, "revision": doc["revision"], "status": doc["status"],
            "errors": len(v["errors"]), "warnings": len(v["warnings"]), "unsupported": len(v["unsupported"])}


def do_activate(db: Database, req: dict, fx: Effects) -> dict:
    p = req["params"]
    mid = _model_id(p)
    doc = mstore.activate(db, mid, int(p.get("revision", 0)))
    fx.reload_models.add(mid)
    return {"modelId": mid, "activeRevision": doc["revision"]}


def do_rollback(db: Database, req: dict, fx: Effects) -> dict:
    mid = _model_id(req["params"])
    doc = mstore.rollback(db, mid)
    fx.reload_models.add(mid)
    return {"modelId": mid, "activeRevision": doc["revision"]}


def _review_model(db: Database, p: dict) -> tuple[str, dict]:
    """The revision given in the params, else the active one, else the newest usable one."""
    mid = _model_id(p)
    if p.get("revision"):
        model = db[MODELS].find_one({"modelId": mid, "revision": int(p["revision"])})
    else:
        model = mstore.get_active(db, mid) or db[MODELS].find_one(
            {"modelId": mid, "status": {"$ne": "rejected"}}, sort=[("revision", -1)])
    if model is None:
        raise RequestError(f"{mid}: no usable model revision")
    return mid, model


def do_propose(db: Database, req: dict, fx: Effects) -> dict:
    from .bindings.proposal import propose_analogs, propose_switches

    mid, model = _review_model(db, req["params"])
    net = mstore.load_network(db, model)
    tags = bstore.candidate_tags(db)
    sw, an = propose_switches(net, tags), propose_analogs(net, tags)
    bindings = [asdict(b) for b in sw.bindings + an.bindings]
    art = _put_json(db, bindings, f"proposal_{mid}_{model['revision']}.json", kind="binding-proposal",
                    modelId=mid, requestId=req["_id"])
    switches = len(net.get_switches())
    return {
        "modelId": mid, "modelRevision": model["revision"], "artifactId": str(art), "tags": len(tags),
        "switches": {"total": switches, "proposed": len(sw.bindings), "unmatched": len(sw.unmatched),
                     "ambiguous": len(sw.ambiguous)},
        "analogs": {"proposed": len(an.bindings), "unmatched": len(an.unmatched), "ambiguous": len(an.ambiguous)},
        "unmatched": (sw.unmatched + an.unmatched)[:LIST_LIMIT],
        "ambiguous": (sw.ambiguous + an.ambiguous)[:LIST_LIMIT],
    }


def do_list_elements(db: Database, req: dict, fx: Effects) -> dict:
    """Element catalog of a model revision as a JSON artifact (see `bindings.catalog`)."""
    from .bindings.catalog import element_catalog

    mid, model = _review_model(db, req["params"])
    entries = element_catalog(mstore.load_network(db, model))
    art = _put_json(db, entries, f"elements_{mid}_{model['revision']}.json", kind="element-catalog",
                    modelId=mid, requestId=req["_id"])
    fs = artifacts(db)
    old = db[f"{ARTIFACT_BUCKET}.files"].find({"kind": "element-catalog", "modelId": mid}, {"_id": 1}) \
        .sort("uploadDate", -1).skip(CATALOG_KEEP)
    for d in list(old):
        fs.delete(d["_id"])
    per_kind: dict[str, int] = {}
    for e in entries:
        per_kind[e["kind"]] = per_kind.get(e["kind"], 0) + 1
    return {"modelId": mid, "modelRevision": model["revision"], "artifactId": str(art), "entries": len(entries),
            "perKind": per_kind, "substations": len({e["substation"] for e in entries})}


def do_save_bindings(db: Database, req: dict, fx: Effects) -> dict:
    p = req["params"]
    mid = _model_id(p)
    art = _oid(p.get("artifactId"))
    fs = artifacts(db)
    raw = json.loads(fs.get(art).read())
    if not isinstance(raw, list):
        raise RequestError("binding set must be a list")
    allowed = set(Binding.__dataclass_fields__)
    bindings = []
    for i, d in enumerate(raw):
        try:
            b = Binding(**{k: v for k, v in d.items() if k in allowed})
            b.validate()
        except (TypeError, ValueError) as e:
            raise RequestError(f"binding {i}: {e}") from e
        bindings.append(b)
    kept, own = bstore.drop_own_points(db, bindings)
    if own:
        raise RequestError(f"{len(own)} binding(s) refer to CIM-EMS result points, e.g. {own[0].tag}")
    rev = bstore.save_revision(db, mid, kept, user=req.get("createdBy") or "admin-ui")
    fs.delete(art)
    fx.reload_models.add(mid)
    return {"modelId": mid, "bindingRevision": rev, "count": len(kept), "approved": sum(b.approved for b in kept)}


def do_rerun(db: Database, req: dict, fx: Effects) -> dict:
    n = int(req["params"].get("connectionNumber", 0))
    if n <= 0:
        raise RequestError("invalid connectionNumber")
    fx.rerun.add(n)
    return {"connectionNumber": n}


HANDLERS = {
    "import": do_import,
    "activate": do_activate,
    "rollback": do_rollback,
    "proposeBindings": do_propose,
    "listElements": do_list_elements,
    "saveBindings": do_save_bindings,
    "rerun": do_rerun,
}


def claim(db: Database, node: str) -> dict | None:
    return db[STUDY_REQUESTS].find_one_and_update(
        {"status": "pending"},
        {"$set": {"status": "running", "startedAt": _now(), "nodeName": node}},
        sort=[("createdAt", 1)],
        return_document=ReturnDocument.AFTER,
    )


def fail_stale(db: Database, stale_seconds: float = STALE_SECONDS) -> int:
    r = db[STUDY_REQUESTS].update_many(
        {"status": "running", "startedAt": {"$lt": _now() - timedelta(seconds=stale_seconds)}},
        {"$set": {"status": "failed", "error": "interrupted (driver node stopped)", "finishedAt": _now()}},
    )
    return r.modified_count


def execute(db: Database, req: dict, fx: Effects) -> None:
    handler = HANDLERS.get(req.get("type"))
    upd: dict
    try:
        if handler is None:
            raise RequestError(f"unknown request type {req.get('type')!r}")
        if not isinstance(req.get("params"), dict):
            req["params"] = {}
        result = handler(db, req, fx)
        upd = {"status": "done", "result": result}
        log.info("request %s %s done: %s", req["_id"], req.get("type"), result)
    except (RequestError, mstore.ModelError) as e:
        upd = {"status": "failed", "error": str(e)[:500]}
        log.warning("request %s %s failed: %s", req["_id"], req.get("type"), e)
    except Exception as e:                       # never let a request kill the driver loop
        log.exception("request %s failed", req["_id"])
        upd = {"status": "failed", "error": f"{type(e).__name__}: {str(e)[:400]}"}
    upd["finishedAt"] = _now()
    db[STUDY_REQUESTS].update_one({"_id": req["_id"]}, {"$set": upd})


def process_pending(db: Database, node: str, max_requests: int = 5) -> Effects:
    """Execute up to `max_requests` pending requests, oldest first. Call on the active node only."""
    fx = Effects()
    fail_stale(db)
    for _ in range(max_requests):
        req = claim(db, node)
        if req is None:
            break
        execute(db, req, fx)
    return fx
