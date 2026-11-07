import sys
from pathlib import Path

import pytest

from cim_ems import synthetic
from cim_ems.bindings.proposal import propose_switches
from cim_ems.models.importer import import_cgmes

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "tools"))
from replay import replay  # noqa: E402

SMALL = Path(__file__).resolve().parents[1] / "data/conformity/smallgrid_nodebreaker.zip"
pytestmark = pytest.mark.skipif(not SMALL.exists(), reason="run tools/fetch_test_models.py")


def _bindings(net):
    prop = propose_switches(net, synthetic.switch_tags(net))
    for b in prop.bindings:
        b.approved = True
    return prop


def test_smallgrid_all_switches_match_uniquely():
    net = import_cgmes(SMALL)
    prop = _bindings(net)
    assert len(prop.bindings) == 1107 and not prop.unmatched and not prop.ambiguous


def test_replay_is_deterministic_and_matches_expected_open_count():
    runs = []
    for _ in range(2):
        net = import_cgmes(SMALL)
        prop = _bindings(net)
        events = [(t, tag, v, False, False) for t, tag, v in synthetic.random_events(prop.bindings, 15, seed=7)]
        runs.append(replay(net, prop.bindings, events, run_load_flow=False))
    assert [r["hash"] for r in runs[0]] == [r["hash"] for r in runs[1]]
    assert len({r["hash"] for r in runs[0]}) > 5
    # the model has no open switches initially: open count equals the number of open states after each event
    state = {}
    for r, (_, tag, v, _, _) in zip(runs[0], events):
        state[tag] = v
        assert r["open"] == sum(1 for x in state.values() if x == 0.0)


def test_replay_invalid_then_recovered_keeps_topology_and_flags():
    net = import_cgmes(SMALL)
    prop = _bindings(net)
    tag = prop.bindings[0].tag
    t = synthetic.T0
    from datetime import timedelta
    events = [(t + timedelta(seconds=10), tag, 0.0, False, False),
              (t + timedelta(seconds=20), tag, 1.0, True, False),     # invalid: hold open
              (t + timedelta(seconds=30), tag, 1.0, False, False)]    # valid closed
    rows = replay(net, prop.bindings, events, run_load_flow=False)
    assert rows[0]["open"] == 1
    assert rows[1]["open"] == 1 and "topologyUncertain" in rows[1]["flags"]
    assert rows[2]["open"] == 0 and not rows[2]["flags"]
