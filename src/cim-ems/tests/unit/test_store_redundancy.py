import os
from datetime import datetime, timedelta, timezone

import mongomock
import pypowsybl.network as pn
import pytest

from cim_ems import mongo
from cim_ems.export import export_cgmes
from cim_ems.models import store
from cim_ems.models.importer import import_cgmes, validate
from cim_ems.redundancy import Redundancy


@pytest.fixture
def db():
    """Real MongoDB when CIM_EMS_TEST_MONGO is set (enforces the unique/partial/TTL indexes), else mongomock."""
    uri = os.environ.get("CIM_EMS_TEST_MONGO")
    if uri:
        import pymongo

        client = pymongo.MongoClient(uri)
        d = client["cim_ems_test"]
        client.drop_database("cim_ems_test")
    else:
        d = mongomock.MongoClient()["t"]
    mongo.ensure_indexes(d)
    yield d
    if uri:
        client.drop_database("cim_ems_test")


def test_second_active_revision_rejected_by_index(db, package):
    if not os.environ.get("CIM_EMS_TEST_MONGO"):
        pytest.skip("partial unique index needs a real MongoDB")
    import pymongo

    _register(db, package), _register(db, package)
    db[mongo.MODELS].update_one({"revision": 1}, {"$set": {"status": "active"}})
    with pytest.raises(pymongo.errors.DuplicateKeyError):
        db[mongo.MODELS].update_one({"revision": 2}, {"$set": {"status": "active"}})


@pytest.fixture(scope="module")
def package(tmp_path_factory):
    f = tmp_path_factory.mktemp("c") / "eurostag.zip"
    export_cgmes(pn.create_eurostag_tutorial_example1_network(), f)
    return f


def _register(db, package):
    rep = validate(import_cgmes(package))
    return store.register_import(db, package, "M1", rep)


def test_revisions_increment_and_start_validated(db, package):
    a, b = _register(db, package), _register(db, package)
    assert (a["revision"], b["revision"]) == (1, 2)
    assert a["status"] == "validated" and a["cgmesVersion"] == "2.4.15"
    assert len(a["sourceSha256"]) == 64


def test_single_active_and_rollback(db, package):
    for _ in range(3):
        _register(db, package)
    store.activate(db, "M1", 1)
    store.activate(db, "M1", 2)
    assert store.get_active(db, "M1")["revision"] == 2
    assert db[mongo.MODELS].count_documents({"modelId": "M1", "status": "active"}) == 1
    assert store.rollback(db, "M1")["revision"] == 1
    with pytest.raises(store.ModelError):
        store.rollback(db, "M1")


def test_rejected_cannot_be_activated(db, package):
    rep = validate(import_cgmes(package))
    rep.errors.append("forced")
    doc = store.register_import(db, package, "M2", rep)
    assert doc["status"] == "rejected"
    with pytest.raises(store.ModelError):
        store.activate(db, "M2", 1)


def test_stored_package_rebuilds_network(db, package):
    doc = _register(db, package)
    net = store.load_network(db, doc)
    assert len(net.get_generators()) == len(import_cgmes(package).get_generators())


def _instance(db, nodes=("a", "b")):
    db["protocolDriverInstances"].insert_one(
        {"protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1, "enabled": True,
         "logLevel": 1, "nodeNames": list(nodes), "activeNodeName": "", "activeNodeKeepAliveTimeTag": None}
    )


def test_single_publisher_and_failover(db):
    _instance(db)
    ra, rb = Redundancy(db, "a", 1), Redundancy(db, "b", 1)
    t0 = datetime.now(timezone.utc)
    assert ra.tick(t0) and ra.still_active()
    assert not rb.tick(t0 + timedelta(seconds=5)) and not rb.still_active()
    assert ra.tick(t0 + timedelta(seconds=10))
    # a stops ticking; b takes over after the timeout
    assert rb.tick(t0 + timedelta(seconds=45))
    assert rb.still_active() and not ra.still_active()


def test_disabled_or_foreign_node_never_active(db):
    _instance(db, nodes=("a",))
    assert not Redundancy(db, "z", 1).tick()
    db["protocolDriverInstances"].update_one({}, {"$set": {"enabled": False}})
    assert not Redundancy(db, "a", 1).tick()


def test_empty_node_names_allow_any_node(db):
    _instance(db, nodes=())
    assert Redundancy(db, "z", 1).tick()
