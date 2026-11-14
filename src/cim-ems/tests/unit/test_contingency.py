import pypowsybl.network as pn

from cim_ems.contingency import Contingency, STATUS_CODES, analyse, auto_n1, from_document, to_document, to_result_values


def net():
    return pn.create_eurostag_tutorial_example1_network()


def run(*cs):
    return {o.id: o for o in analyse(net(), list(cs)).outcomes}


def test_classification_on_constructed_cases():
    o = run(Contingency("one", "line 1", ["NHV1_NHV2_1"]),
            Contingency("both", "both lines", ["NHV1_NHV2_1", "NHV1_NHV2_2"]),
            Contingency("gen", "generator", ["GEN"]),
            Contingency("bad", "unknown", ["NOPE"]),
            Contingency("empty", "no elements", []))
    assert o["one"].status == "violation"
    assert {v["type"] for v in o["one"].violations} == {"CURRENT", "LOW_VOLTAGE"}
    assert o["both"].status == "islanding" and "LOAD" in o["both"].disconnected
    assert o["gen"].status == "secure" and o["gen"].worst_loading < 100
    assert o["bad"].status == "unsupported" and o["empty"].status == "unsupported"


def test_overload_loading_percent():
    o = run(Contingency("one", "line 1", ["NHV1_NHV2_1"]))["one"]
    assert 200 < o.worst_loading < 220      # 1048 A on a 500 A permanent limit


def test_preexisting_violation_is_not_blamed_on_the_outage():
    n = net()
    n.update_voltage_levels(id="VLHV1", low_voltage_limit=420.0)   # the base case is already below the limit
    res = analyse(n, [Contingency("gen", "generator", ["GEN"])])
    assert any(v["type"] == "LOW_VOLTAGE" for v in res.base_violations)
    assert res.outcomes[0].status == "secure" and not res.outcomes[0].violations


def test_auto_n1_lists_in_service_branches_and_filters_voltage():
    n = net()
    ids = {c.id for c in auto_n1(n)}
    assert ids == {"NHV1_NHV2_1", "NHV1_NHV2_2", "NGEN_NHV1", "NHV2_NLOAD"}
    assert {c.id for c in auto_n1(n, 300)} == {"NHV1_NHV2_1", "NHV1_NHV2_2", "NHV2_NLOAD"}


def test_result_values_and_document_round_trip():
    res = analyse(net(), from_document({"contingencies": [{"id": "a", "elements": ["NHV1_NHV2_1"]},
                                                          {"id": "b", "name": "gen", "elements": ["GEN"]}]}))
    vals = {(v.element_id, v.quantity): v.value for v in to_result_values(res)}
    assert vals[("a", "STATUS")] == STATUS_CODES["violation"] and vals[("b", "STATUS")] == 0.0
    assert vals[("SYSTEM", "COUNT")] == 2.0 and vals[("SYSTEM", "INSECURE")] == 1.0
    assert vals[("SYSTEM", "WORST")] > 200
    assert [o["id"] for o in to_document(res)["outcomes"]] == ["a", "b"]
    assert len(to_result_values(res, publish_limit=1)) == 3 + 4
