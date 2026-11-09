import json
from datetime import datetime, timedelta, timezone

import mongomock
import pypowsybl.network as pn
import pytest

from cim_ems import api, mongo, synthetic
from cim_ems.bindings import store as bstore
from cim_ems.export import export_cgmes


@pytest.fixture(scope="module")
def package(tmp_path_factory):
    f = tmp_path_factory.mktemp("api") / "four_substations.zip"
    export_cgmes(pn.create_four_substations_node_breaker_network(), f)
    return f


@pytest.fixture
def db():
    d = mongomock.MongoClient()["t"]
    mongo.ensure_indexes(d)
    return d


def _request(db, type_, params, created=None):
    db[mongo.STUDY_REQUESTS].insert_one({"type": type_, "params": params, "status": "pending",
                                         "createdAt": created or datetime.now(timezone.utc), "createdBy": "eng"})


def _upload(db, package, model_id="M1"):
    art = mongo.artifacts(db).put(package.read_bytes(), filename=package.name, kind="upload")
    _request(db, "import", {"modelId": model_id, "artifactId": str(art), "fileName": package.name})
    return art


def _req(db, type_):
    return db[mongo.STUDY_REQUESTS].find_one({"type": type_}, sort=[("createdAt", -1)])


def test_import_registers_a_validated_revision_and_drops_the_upload(db, package):
    art = _upload(db, package)
    api.process_pending(db, "n1")
    r = _req(db, "import")
    assert r["status"] == "done" and r["nodeName"] == "n1" and r["finishedAt"]
    assert r["result"]["revision"] == 1 and r["result"]["status"] == "validated"
    m = db[mongo.MODELS].find_one({"modelId": "M1"})
    assert m["importedBy"] == "eng" and m["status"] == "validated"    # activation stays explicit
    assert not mongo.artifacts(db).exists(art)
    assert mongo.artifacts(db).exists(m["sourceArtifactId"])


def test_garbage_upload_fails_without_a_revision(db, tmp_path):
    bad = tmp_path / "bad.zip"
    bad.write_bytes(b"PK\x03\x04 not really a zip")
    _upload(db, bad)
    api.process_pending(db, "n1")
    r = _req(db, "import")
    assert r["status"] == "failed" and r["error"]
    assert db[mongo.MODELS].count_documents({}) == 0


def test_activate_rollback_and_reload_effects(db, package):
    _upload(db, package)
    _upload(db, package, "M1")
    api.process_pending(db, "n1")
    _request(db, "activate", {"modelId": "M1", "revision": 1})
    fx = api.process_pending(db, "n1")
    assert fx.reload_models == {"M1"} and _req(db, "activate")["result"]["activeRevision"] == 1
    _request(db, "activate", {"modelId": "M1", "revision": 2})
    api.process_pending(db, "n1")
    _request(db, "rollback", {"modelId": "M1"})
    api.process_pending(db, "n1")
    assert _req(db, "rollback")["result"]["activeRevision"] == 1
    _request(db, "activate", {"modelId": "M1", "revision": 9})
    api.process_pending(db, "n1")
    r = _req(db, "activate")
    assert r["status"] == "failed" and "not found" in r["error"]


def test_propose_then_save_bindings(db, package):
    _upload(db, package)
    api.process_pending(db, "n1")
    net = pn.create_four_substations_node_breaker_network()
    tags = synthetic.switch_tags(net) + synthetic.analog_tags(net)[0]
    db["realtimeData"].insert_many([{**t, "origin": "supervised", "protocolSourceConnectionNumber": 5.0}
                                    for t in tags])
    _request(db, "proposeBindings", {"modelId": "M1"})
    api.process_pending(db, "n1")
    r = _req(db, "proposeBindings")
    assert r["status"] == "done", r.get("error")
    res = r["result"]
    assert res["modelRevision"] == 1 and res["switches"]["proposed"] == res["switches"]["total"] > 0
    assert res["analogs"]["proposed"] > 0
    proposal = json.loads(mongo.artifacts(db).get(api._oid(res["artifactId"])).read())
    assert all(not b["approved"] for b in proposal)

    for b in proposal[:10]:
        b["approved"] = True
    art = mongo.artifacts(db).put(json.dumps(proposal).encode(), filename="b.json")
    _request(db, "saveBindings", {"modelId": "M1", "artifactId": str(art)})
    fx = api.process_pending(db, "n1")
    r = _req(db, "saveBindings")
    assert r["status"] == "done" and r["result"]["bindingRevision"] == 1 and r["result"]["approved"] == 10
    assert fx.reload_models == {"M1"}
    assert len(bstore.load_revision(db, "M1", 1)) == 10
    assert len(bstore.load_revision(db, "M1", 1, approved_only=False)) == len(proposal)


def test_save_refuses_own_points_and_bad_entries(db):
    db["protocolConnections"].insert_one({"protocolDriver": "CIM-EMS", "protocolConnectionNumber": 9})
    db["realtimeData"].insert_one({"tag": "9;SE;L1;P;1", "protocolSourceConnectionNumber": 9.0})
    fs = mongo.artifacts(db)
    own = [{"kind": "analog", "element_id": "L1", "tag": "9;SE;L1;P;1", "quantity": "P", "side": 1, "approved": True}]
    _request(db, "saveBindings", {"modelId": "M1", "artifactId": str(fs.put(json.dumps(own).encode()))})
    bad = [{"kind": "analog", "element_id": "L1", "tag": "X", "quantity": "W"}]
    _request(db, "saveBindings", {"modelId": "M1", "artifactId": str(fs.put(json.dumps(bad).encode()))})
    api.process_pending(db, "n1")
    errs = [r["error"] for r in db[mongo.STUDY_REQUESTS].find().sort("createdAt")]
    assert "CIM-EMS result points" in errs[0] and "binding 0" in errs[1]
    assert bstore.latest_revision(db, "M1") is None


def test_rerun_unknown_type_and_stale_requests(db):
    old = datetime.now(timezone.utc) - timedelta(hours=2)
    db[mongo.STUDY_REQUESTS].insert_one({"type": "rerun", "params": {}, "status": "running", "startedAt": old,
                                         "createdAt": old})
    _request(db, "rerun", {"connectionNumber": 7})
    _request(db, "drop", {})
    fx = api.process_pending(db, "n1")
    assert fx.rerun == {7}
    st = {r["type"]: (r["status"], r.get("error", "")) for r in db[mongo.STUDY_REQUESTS].find({"status": {"$ne": "running"}})}
    assert st["drop"][0] == "failed" and "unknown request type" in st["drop"][1]
    assert db[mongo.STUDY_REQUESTS].count_documents({"status": "failed", "error": {"$regex": "interrupted"}}) == 1


def test_requests_run_oldest_first(db):
    t = datetime.now(timezone.utc)
    _request(db, "rerun", {"connectionNumber": 2}, t)
    _request(db, "rerun", {"connectionNumber": 1}, t - timedelta(seconds=5))
    first = api.claim(db, "n1")
    assert first["params"]["connectionNumber"] == 1


def test_list_elements_catalog(db, package):
    from cim_ems.bindings.proposal import analog_labels, switch_labels
    from cim_ems.models import store as mstore

    _upload(db, package)
    api.process_pending(db, "n1")
    _request(db, "listElements", {"modelId": "M1"})
    api.process_pending(db, "n1")
    r = _req(db, "listElements")
    assert r["status"] == "done", r.get("error")
    res = r["result"]
    entries = json.loads(mongo.artifacts(db).get(api._oid(res["artifactId"])).read())
    assert res["entries"] == len(entries) and res["modelRevision"] == 1
    net = mstore.load_network(db, db[mongo.MODELS].find_one({"modelId": "M1"}))
    sw = {e["element_id"]: e for e in entries if e["kind"] == "switch"}
    assert sw and {k: e["label"] for k, e in sw.items()} == switch_labels(net)
    an = {(e["element_id"], e["side"], e["quantity"]): e for e in entries if e["kind"] == "analog"}
    assert {k: e["label"] for k, e in an.items()} == analog_labels(net)
    branch_end = next(e for e in an.values() if e["side"] == 1)
    assert branch_end["elementType"] in ("line", "transformer") and branch_end["otherEnd"]["substation"] is not None
    assert all(e["unit"] in ("kV", "MW", "Mvar") for e in an.values())
    taps = [e for e in entries if e["kind"] == "tap"]
    assert all(e["tapRange"][0] <= e["tap"] <= e["tapRange"][1] for e in taps)

    for _ in range(api.CATALOG_KEEP + 2):     # older catalogs of the model are pruned
        _request(db, "listElements", {"modelId": "M1"})
        api.process_pending(db, "n1")
    assert db["cimArtifacts.files"].count_documents({"kind": "element-catalog", "modelId": "M1"}) == api.CATALOG_KEEP
