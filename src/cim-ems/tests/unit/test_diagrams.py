import json
import xml.etree.ElementTree as ET

import pypowsybl.loadflow as lf
import pypowsybl.network as pn

from cim_ems.bindings.model import Binding
from cim_ems.diagrams.generate import generate_all
from cim_ems.diagrams.sld import LABEL, switch_labels
from cim_ems.results import extract_loadflow


def test_switch_labels_closed_on_vs_off():
    c_on, o_on = switch_labels("T", True)
    c_off, o_off = switch_labels("T", False)
    d = lambda s: {e["data"]: e["param"] for e in json.loads("[" + s + "]")[0]["list"]}
    assert "visible" in d(c_on)["2"] and "hidden" in d(c_on)["1"]
    assert "hidden" in d(o_on)["2"] and "visible" in d(o_on)["1"]
    assert "visible" in d(c_off)["1"] and "hidden" in d(c_off)["2"]
    assert "visible" in d(c_on)["0"] is False or "hidden" in d(c_on)["0"]   # unknown: open drawing with amber
    assert "f0ad00" in d(o_on)["0"]


def test_generate_four_substations(tmp_path):
    net = pn.create_four_substations_node_breaker_network()
    lf.run_ac(net)
    sws = net.get_switches()
    bindings = [Binding(kind="switch", element_id=s, tag=f"SW.{i}", approved=True) for i, s in enumerate(sws.index)
                if i % 2 == 0]
    reps = generate_all(net, "CIM1", bindings, extract_loadflow(net), tmp_path)
    assert len(list(tmp_path.glob("*.svg"))) == len(net.get_voltage_levels())
    assert sum(r.switches_bound for r in reps) == len(bindings)
    assert sum(len(r.switches_unbound) for r in reps) == len(sws) - len(bindings)
    assert sum(r.values_bound for r in reps) > 0
    tags = {b.tag for b in bindings}
    n = 0
    for f in tmp_path.glob("*.svg"):
        for e in ET.parse(f).getroot().iter():
            lab = e.get(LABEL)
            if lab:
                n += 1
                for obj in json.loads("[" + lab + "]"):
                    for ent in obj.get("list", []) + [obj]:
                        t = ent.get("tag")
                        if t and t.startswith("SW."):
                            assert t in tags
    assert n > 0


def test_names_shown_and_unbound_values_removed(tmp_path):
    net = pn.create_four_substations_node_breaker_network()
    lf.run_ac(net)
    generate_all(net, "CIM1", [], extract_loadflow(net), tmp_path, only=["S1VL2"])
    root = ET.parse(next(tmp_path.glob("*.svg"))).getroot()
    texts = ["".join(t.itertext()) for t in root.iter("{http://www.w3.org/2000/svg}text")]
    gen_names = set(net.get_generators().name.dropna()) | set(net.get_loads().name.dropna())
    assert any(t in gen_names for t in texts) or any("S1VL2" in t for t in texts)
    # generator and load feeder values have no result point: no static numbers left
    static = [t for t in texts if t.strip().lstrip("-").replace(".", "").isdigit()]
    assert static == []
