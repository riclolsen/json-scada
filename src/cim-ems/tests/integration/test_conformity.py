"""Regression on public CGMES 2.4.15 conformity models. Skipped unless downloaded:
python tools/fetch_test_models.py
"""
import os
from pathlib import Path

import pypowsybl.loadflow as lf
import pytest

from cim_ems.export import round_trip
from cim_ems.models.importer import import_cgmes, validate

DATA = Path(os.environ.get("CIM_EMS_TEST_DATA", Path(__file__).resolve().parents[1] / "data/conformity"))
SMALL = DATA / "smallgrid_nodebreaker.zip"
MICRO = DATA / "microgrid_basecase.zip"
needs = pytest.mark.skipif(not SMALL.exists(), reason="run tools/fetch_test_models.py")


@needs
def test_smallgrid_node_breaker_validates():
    net = import_cgmes(SMALL)
    rep = validate(net)
    assert rep.ok and rep.load_flow["status"] == "CONVERGED"
    assert rep.counts["switches"] == 1107 and rep.counts["substations"] == 105
    assert rep.islands == 1


@needs
def test_smallgrid_switching_changes_topology_and_reconverges():
    net = import_cgmes(SMALL)
    lf.run_ac(net)
    sw = net.get_switches()
    breaker = sw[(~sw.open) & (sw.kind == "BREAKER")].index[0]
    before = len(net.get_buses())
    net.update_switches(id=breaker, open=True)
    assert lf.run_ac(net)[0].status.name == "CONVERGED"
    net.update_switches(id=breaker, open=False)
    assert len(net.get_buses()) == before


@needs
def test_smallgrid_round_trip_with_boundary():
    res = round_trip(import_cgmes(SMALL), boundary_package=SMALL)
    assert res["ok"], res["problems"]


@pytest.mark.skipif(not MICRO.exists(), reason="run tools/fetch_test_models.py")
def test_microgrid_round_trip_within_tolerance():
    res = round_trip(import_cgmes(MICRO), boundary_package=MICRO)
    assert res["ok"], res["problems"]
    assert validate(import_cgmes(MICRO)).counts["three_windings_transformers"] == 1


RELICAP = {r: DATA / f"relicap_{r}.zip" for r in
           ("Belgovia", "Britheim", "Galia", "Nordheim", "Portheim", "Svedala", "Espheim", "DC-Espheim-Svedala", "DC-Nordheim-Galia")}
relicap = pytest.mark.skipif(not RELICAP["Galia"].exists(), reason="run tools/fetch_test_models.py --relicap")


@relicap
@pytest.mark.parametrize("region", ["Belgovia", "Galia", "Portheim", "Svedala", "Espheim", "DC-Nordheim-Galia"])
def test_relicap_regions_validate_and_round_trip_exactly(region):
    pkg = RELICAP[region]
    net = import_cgmes(pkg)
    rep = validate(net)
    assert rep.ok, rep.errors
    res = round_trip(net, boundary_package=pkg)
    assert res["ok"], res["problems"]
    assert res["max_dp_mw"] < 1e-6 and res["max_dq_mvar"] < 1e-6


@relicap
def test_relicap_node_breaker_sizes():
    rep = validate(import_cgmes(RELICAP["Svedala"]))
    assert rep.counts["switches"] == 1470 and rep.counts["voltage_levels"] == 99
    assert rep.islands == 6   # six synchronous components, only the main one is calculated


@relicap
def test_relicap_vsc_hvdc_in_ac_side_flags_and_loses_reactive_setpoint():
    pkg = RELICAP["Britheim"]
    net = import_cgmes(pkg)
    rep = validate(net)
    assert any("VSC" in u for u in rep.unsupported)
    assert not round_trip(net, boundary_package=pkg)["ok"]


@relicap
def test_relicap_lcc_only_network_is_not_calculated_and_explained():
    rep = validate(import_cgmes(RELICAP["DC-Espheim-Svedala"]))
    assert not rep.ok and any("LCC" in u for u in rep.unsupported)


@relicap
def test_relicap_nordheim_has_nothing_to_calculate():
    rep = validate(import_cgmes(RELICAP["Nordheim"]))
    assert not rep.ok and any("no network component" in e for e in rep.errors)
