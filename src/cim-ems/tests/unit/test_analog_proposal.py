import mongomock
import pypowsybl.network as pn

from cim_ems import synthetic
from cim_ems.bindings.model import Binding
from cim_ems.bindings.proposal import analog_labels, propose_analogs, quantity_of
from cim_ems.bindings.store import candidate_tags, drop_own_points


def test_quantity_from_unit_then_suffix_then_description():
    assert quantity_of({"tag": "SUB;L1;X", "unit": "MW"}) == "P"
    assert quantity_of({"tag": "SUB;VL 132KV;L1;Q"}) == "Q"          # the last quantity word wins over "132kv"
    assert quantity_of({"tag": "SUB;BB1;V"}) == "V"
    assert quantity_of({"tag": "SUB;L1;X", "description": "reactive power"}) == "Q"
    assert quantity_of({"tag": "SUB;L1;TEMP"}) is None


def net():
    return pn.create_four_substations_node_breaker_network()


def test_synthetic_tags_are_bound_exactly_or_left_ambiguous():
    n = net()
    for style in ("letter", "unit"):
        tags, truth = synthetic.analog_tags(n, style)
        prop = propose_analogs(n, tags)
        assert prop.bindings
        assert all(truth[b.tag] == (b.element_id, b.side, b.quantity) for b in prop.bindings)
        assert all(b.kind == "analog" and not b.approved and b.proposed_from == "name-match" for b in prop.bindings)


def test_voltage_sigma_scales_with_nominal_voltage():
    n = net()
    tags, _ = synthetic.analog_tags(n)
    v = [b for b in propose_analogs(n, tags).bindings if b.quantity == "V"]
    nominal = n.get_voltage_levels()["nominal_v"]
    bbs = n.get_busbar_sections()["voltage_level_id"]
    assert v and all(abs(b.sigma - 0.004 * nominal[bbs[b.element_id]]) < 1e-9 for b in v)


def test_identical_labels_are_ambiguous_not_guessed():
    n = net()
    line = n.get_lines().loc["LINE_S2S3"]
    n.create_lines(id="TWIN", name=line["name"] or "LINE_S2S3", voltage_level1_id=line.voltage_level1_id,
                   voltage_level2_id=line.voltage_level2_id, node1=90, node2=90, r=1, x=10, g1=0, b1=0, g2=0, b2=0)
    label = analog_labels(n)[("LINE_S2S3", 1, "P")]
    assert analog_labels(n)[("TWIN", 1, "P")] == label          # a parallel circuit with the same name
    prop = propose_analogs(n, [{"_id": 1.0, "tag": ";".join(label.split(" ")) + ";P", "type": "analog"}])
    assert not prop.bindings
    assert {"LINE_S2S3|1|P", "TWIN|1|P"} <= set(prop.ambiguous)


def test_digital_and_unknown_quantity_tags_are_ignored():
    n = net()
    tags, _ = synthetic.analog_tags(n)
    prop = propose_analogs(n, [dict(t, type="digital") for t in tags] + [{"tag": "X;TEMP", "type": "analog"}])
    assert not prop.bindings


def test_cim_ems_points_are_never_candidates_or_bindings():
    db = mongomock.MongoClient()["t"]
    db["protocolConnections"].insert_one({"protocolDriver": "CIM-EMS", "protocolConnectionNumber": 9001})
    db["realtimeData"].insert_many([
        {"_id": 1.0, "tag": "FIELD;P", "type": "analog", "origin": "supervised", "protocolSourceConnectionNumber": 10.0},
        {"_id": 2.0, "tag": "EMS;SE;L1;P;1", "type": "analog", "origin": "supervised",
         "protocolSourceConnectionNumber": 9001.0},
        {"_id": 3.0, "tag": "CALC", "type": "analog", "origin": "calculated", "protocolSourceConnectionNumber": 0.0}])
    assert [t["tag"] for t in candidate_tags(db)] == ["FIELD;P"]
    kept, own = drop_own_points(db, [Binding("analog", "L1", "FIELD;P", "P", 1),
                                     Binding("analog", "L1", "EMS;SE;L1;P;1", "P", 1)])
    assert [b.tag for b in kept] == ["FIELD;P"] and [b.tag for b in own] == ["EMS;SE;L1;P;1"]
