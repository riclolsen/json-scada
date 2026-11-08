from datetime import datetime, timezone

import mongomock
import pypowsybl.loadflow as lf
import pypowsybl.network as pn
import pytest

from cim_ems import demo
from cim_ems.demo_field import FieldSimulator


@pytest.fixture
def setup():
    net = pn.create_four_substations_node_breaker_network()
    lf.run_ac(net)
    points = demo.field_points(net)
    db = mongomock.MongoClient()["t"]
    db["realtimeData"].insert_many([demo.point_document(p, 91) for p in points])
    return net, points, db


def test_field_points_are_unique_and_linked(setup):
    net, points, _ = setup
    assert len({p.tag for p in points}) == len(points) and len({p.key for p in points}) == len(points)
    sw = [p for p in points if p.kind == "switch"]
    cmds = [p for p in points if p.kind == "command"]
    assert len(sw) == len(net.get_switches())
    assert len(cmds) == (net.get_switches().kind == "BREAKER").sum()
    by_key = {p.key: p for p in points}
    for c in cmds:
        assert by_key[c.supervised_key].command_key == c.key and by_key[c.supervised_key].element_id == c.element_id
    bindings = demo.bindings_for(net, points)
    assert all(b.approved for b in bindings) and len(bindings) == len(points) - len(cmds)
    v = [b for b in bindings if b.quantity == "V"]
    nominal = net.get_voltage_levels().nominal_v
    vl_of = net.get_busbar_sections().voltage_level_id
    assert v and all(abs(b.sigma - demo.SIGMA_V_REL * nominal[vl_of[b.element_id]]) < 1e-3 for b in v)


def test_simulator_publishes_measurements_and_switches(setup):
    net, points, db = setup
    sim = FieldSimulator(db, pn.create_four_substations_node_breaker_network(), 91)
    out = sim.step()
    assert out["converged"]
    meas = demo.Measurer(net)
    for p in points:
        upd = db["realtimeData"].find_one({"_id": p.key})["sourceDataUpdate"]
        if p.kind == "command":
            assert upd is None
        elif p.kind == "switch":
            assert upd["valueAtSource"] == (0.0 if net.get_switches().loc[p.element_id, "open"] else 1.0)
        else:
            true = meas.value(p.element_id, p.side, p.quantity)
            if true is None:
                assert upd["invalidAtSource"]
            else:
                assert not upd["invalidAtSource"] and abs(upd["valueAtSource"] - true) < max(5.0, 0.1 * abs(true))


def test_simulator_executes_and_acks_breaker_command(setup):
    net, points, db = setup
    sim = FieldSimulator(db, pn.create_four_substations_node_breaker_network(), 91)
    sim.step()
    cmd = next(p for p in points if p.kind == "command" and not net.get_switches().loc[p.element_id, "open"])
    now = datetime.now(timezone.utc)
    db["commandsQueue"].insert_many([
        {"protocolSourceConnectionNumber": 91.0, "pointKey": cmd.key, "tag": cmd.tag, "value": 0.0, "timeTag": now},
        {"protocolSourceConnectionNumber": 91.0, "pointKey": 1.0, "tag": "x", "value": 1.0, "timeTag": now},
        {"protocolSourceConnectionNumber": 61.0, "pointKey": cmd.key, "tag": cmd.tag, "value": 1.0, "timeTag": now},
    ])
    assert sim.poll_commands() == 1
    acks = {d["tag"] + str(d["protocolSourceConnectionNumber"]): d.get("ack") for d in db["commandsQueue"].find()}
    assert acks == {cmd.tag + "91.0": True, "x91.0": False, cmd.tag + "61.0": None}
    status = db["realtimeData"].find_one({"_id": cmd.supervised_key})["sourceDataUpdate"]
    assert status["valueAtSource"] == 0.0 and status["causeOfTransmissionAtSource"] == "3"
    assert sim.poll_commands() == 0
