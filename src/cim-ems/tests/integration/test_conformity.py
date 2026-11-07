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
