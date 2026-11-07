"""State estimation on public models: network transfer gate, noisy estimate, bad data, observability."""
import os
from pathlib import Path

import numpy as np
import pypowsybl.loadflow as lf
import pypowsybl.network as pn
import pytest

from cim_ems.estimation import adapter as A
from cim_ems.estimation import synthetic as S
from cim_ems.estimation import transfer as T
from cim_ems.models.importer import import_cgmes

DATA = Path(os.environ.get("CIM_EMS_TEST_DATA", Path(__file__).resolve().parents[1] / "data/conformity"))
needs = pytest.mark.skipif(not (DATA / "relicap_Galia.zip").exists(), reason="run tools/fetch_test_models.py --relicap")


def load(name):
    net = import_cgmes(DATA / f"relicap_{name}.zip")
    lf.run_ac(net)
    return net


def test_eurostag_transfer_matches_load_flow_exactly():
    net = pn.create_eurostag_tutorial_example1_network()
    lf.run_ac(net)
    r = T.compare_load_flow(net, T.to_pandapower(net))
    assert r["max_dv_pu"] < 1e-6 and r["max_dangle_deg"] < 1e-3


def test_unsupported_features_are_rejected():
    net = pn.create_ieee14()   # lines between different nominal voltages
    lf.run_ac(net)
    with pytest.raises(T.UnsupportedForEstimation):
        T.to_pandapower(net)


@needs
@pytest.mark.parametrize("name,dv,da", [("Svedala", 2e-3, 0.1), ("Galia", 2e-3, 0.1)])
def test_transfer_gate_on_cgmes_models(name, dv, da):
    net = load(name)
    r = T.compare_load_flow(net, T.to_pandapower(net))
    assert r["max_dv_pu"] < dv and r["max_dangle_deg"] < da


@needs
def test_noisy_estimate_close_to_truth_and_consistent():
    net = load("Svedala")
    tr = T.to_pandapower(net)
    res = A.estimate(tr, S.from_load_flow(net, tr, np.random.default_rng(3)))
    assert res.converged and res.observable and not res.bad_data
    assert res.chi2_passed
    buses = net.get_buses()
    err = max(abs(res.bus_v_pu[b] - buses.loc[b, "v_mag"] / tr.net.bus.vn_kv[i]) for b, i in tr.bus_index.items())
    assert err < 6e-3


@needs
def test_gross_error_is_identified():
    net = load("Svedala")
    tr = T.to_pandapower(net)
    ms = S.from_load_flow(net, tr, np.random.default_rng(3))
    k = next(i for i, m in enumerate(ms) if m.quantity == "P" and abs(m.value) > 20)
    ms[k].value += 60.0
    res = A.estimate(tr, ms)
    assert res.converged
    assert ms[k].tag in [d.measurement.tag for d in res.bad_data]


@needs
def test_voltage_only_is_not_observable():
    net = load("Galia")
    tr = T.to_pandapower(net)
    ms = [m for m in S.from_load_flow(net, tr) if m.quantity == "V"]
    res = A.estimate(tr, ms)
    assert not res.converged and not res.observable
