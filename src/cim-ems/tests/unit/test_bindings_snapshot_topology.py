import sys
from datetime import timedelta
from pathlib import Path

import pypowsybl.network as pn
import pytest

from cim_ems import synthetic
from cim_ems.bindings.model import Binding, read_csv, write_csv
from cim_ems.bindings.proposal import propose_switches, switch_parts, tokens
from cim_ems.snapshot import PointState, SnapshotBuilder, SnapshotPolicy
from cim_ems.topology import apply_snapshot, topology_hash

T0 = synthetic.T0


def ps(value, **kw):
    kw.setdefault("time_tag", T0)
    kw.setdefault("time_tag_at_source", T0)
    kw.setdefault("time_tag_at_source_ok", True)
    return PointState(value, **kw)


@pytest.fixture
def net():
    return pn.create_four_substations_node_breaker_network()


def sw_binding(tag="S1", eid="SW1", **kw):
    return Binding(kind="switch", element_id=eid, tag=tag, approved=True, **kw)


# --- proposal -------------------------------------------------------------

def test_tokens_split_letters_digits_and_keep_whole_words():
    t = tokens("Annúminas;VLA132KV;BREAKER12")
    assert {"annuminas", "vla", "132", "kv", "breaker", "12", "w:breaker12", "w:vla132kv"} <= t


def test_proposal_matches_exact_names_and_never_approves(net):
    prop = propose_switches(net, synthetic.switch_tags(net))
    assert len(prop.bindings) == len(net.get_switches())
    assert not prop.unmatched and not prop.ambiguous
    assert not any(b.approved for b in prop.bindings)
    assert all(b.proposed_from == "name-match" and b.confidence > 0.9 for b in prop.bindings)


def test_proposal_reports_unmatched_and_ambiguous(net):
    tags = synthetic.switch_tags(net)
    removed = tags.pop(0)
    dup = dict(tags[0], tag=tags[0]["tag"] + "_COPY", _id=9999.0)   # same label twice -> ambiguous
    prop = propose_switches(net, tags + [dup])
    assert len(prop.ambiguous) == 1
    assert removed["tag"] not in {b.tag for b in prop.bindings}
    # the element whose tag was removed must not take a neighbour's tag
    assert len(prop.unmatched) >= 1
    owner = next(sid for sid, p in switch_parts(net).items() if p[2] == removed["tag"].split(";")[-1])
    assert owner in prop.unmatched and owner not in {b.element_id for b in prop.bindings}


def test_proposal_ignores_analog_tags(net):
    tags = [dict(t, type="analog") for t in synthetic.switch_tags(net)]
    assert not propose_switches(net, tags).bindings


def test_csv_round_trip(tmp_path):
    bs = [sw_binding(point_key=3.0), Binding(kind="analog", element_id="L1", tag="P1", quantity="P", side=1, sigma=2.0)]
    write_csv(bs, tmp_path / "b.csv")
    assert read_csv(tmp_path / "b.csv") == bs
    with pytest.raises(ValueError):
        Binding(kind="analog", element_id="x", tag="t").validate()


# --- snapshot rules -------------------------------------------------------

def test_valid_switch_values_and_inverted_closed_value():
    b = SnapshotBuilder([sw_binding("A"), sw_binding("B", "SW2", closed_value=0.0)])
    s = b.build({"A": ps(1.0), "B": ps(1.0)}, T0)
    assert s.switches == {"SW1": False, "SW2": True}   # A closed; B closed means value 0, so 1 is open
    assert not s.flags and not s.rejected


def test_transient_keeps_last_valid_and_flags():
    b = SnapshotBuilder([sw_binding("A")])
    b.build({"A": ps(1.0)}, T0)
    s = b.build({"A": ps(0.0, transient=True)}, T0 + timedelta(seconds=1))
    assert s.switches == {"SW1": False} and "topologyUncertain" in s.flags and not s.rejected


def test_invalid_held_then_rejected_after_limit():
    b = SnapshotBuilder([sw_binding("A")], SnapshotPolicy(max_hold_seconds=60))
    b.build({"A": ps(0.0)}, T0)
    s = b.build({"A": ps(0.0, invalid=True)}, T0 + timedelta(seconds=30))
    assert s.switches == {"SW1": True} and not s.rejected and "topologyUncertain" in s.flags
    s = b.build({"A": ps(0.0, invalid=True)}, T0 + timedelta(seconds=61))
    assert s.rejected


def test_invalid_without_history_keeps_model_state_never_assumes_closed():
    s = SnapshotBuilder([sw_binding("A")]).build({"A": ps(None, invalid=True)}, T0)
    assert "SW1" not in s.switches and "topologyUncertain" in s.flags and not s.rejected


def test_missing_or_nonbinary_switch_value_is_invalid():
    b = SnapshotBuilder([sw_binding("A")])
    assert "SW1" not in b.build({}, T0).switches
    assert "SW1" not in b.build({"A": ps(2.0)}, T0).switches


def test_taps_range_and_integer_check():
    b = SnapshotBuilder([Binding(kind="tap", element_id="T", tag="TAP", approved=True)], tap_ranges={"T": (1, 5)})
    assert b.build({"TAP": ps(3.0)}, T0).taps == {"T": 3}
    for bad in (9.0, 2.5):   # out of range / not an integer: invalid, nothing known to hold
        fresh = SnapshotBuilder([Binding(kind="tap", element_id="T", tag="TAP", approved=True)], tap_ranges={"T": (1, 5)})
        s = fresh.build({"TAP": ps(bad)}, T0)
        assert s.taps == {} and "topologyUncertain" in s.flags
    s = b.build({"TAP": ps(9.0)}, T0)           # after a valid sample the last valid position is held
    assert s.taps == {"T": 3}


def test_analog_age_invalid_skew_and_sign():
    bs = [Binding(kind="analog", element_id=f"L{i}", tag=f"P{i}", quantity="P", side=1, sign=-1.0 if i == 1 else 1.0,
                  approved=True, max_age_seconds=30) for i in range(1, 5)]
    pts = {
        "P1": ps(10.0),
        "P2": ps(20.0, invalid=True),
        "P3": ps(30.0, time_tag_at_source=T0 - timedelta(seconds=60)),   # too old
        "P4": ps(40.0, time_tag_at_source=T0 - timedelta(seconds=20)),   # skewed, not old
    }
    s = SnapshotBuilder(bs, SnapshotPolicy(max_skew_seconds=5)).build(pts, T0)
    assert [(m.element_id, m.value) for m in s.measurements] == [("L1", -10.0)]


def test_substituted_analog_is_used_and_flagged():
    b = SnapshotBuilder([Binding(kind="analog", element_id="L", tag="P", quantity="P", approved=True)])
    s = b.build({"P": ps(5.0, substituted=True)}, T0)
    assert len(s.measurements) == 1 and "substituted" in s.flags


def test_unapproved_bindings_are_ignored():
    s = SnapshotBuilder([Binding(kind="switch", element_id="SW1", tag="A")]).build({"A": ps(0.0)}, T0)
    assert not s.switches


# --- topology -------------------------------------------------------------

def test_apply_snapshot_changes_switches_and_hash_is_reversible(net):
    h0 = topology_hash(net)
    sid = net.get_switches().query("kind == 'BREAKER' and not open").index[0]
    b = SnapshotBuilder([sw_binding("A", sid)])
    info = apply_snapshot(net, b.build({"A": ps(0.0)}, T0))     # 0 = open
    assert info.changed_switches == 1 and info.hash != h0
    info = apply_snapshot(net, b.build({"A": ps(1.0)}, T0))
    assert info.hash == h0 and info.changed_switches == 1
    assert apply_snapshot(net, b.build({"A": ps(1.0)}, T0)).changed_switches == 0


def test_unknown_switch_is_flagged_not_fatal(net):
    b = SnapshotBuilder([sw_binding("A", "NOT_IN_MODEL")])
    snap = b.build({"A": ps(0.0)}, T0)
    apply_snapshot(net, snap)
    assert "bindingMismatch" in snap.flags


def test_tap_applied(net):
    tid = net.get_ratio_tap_changers().index[0]
    cur = int(net.get_ratio_tap_changers().loc[tid, "tap"])
    b = SnapshotBuilder([Binding(kind="tap", element_id=tid, tag="TAP", approved=True)])
    snap = b.build({"TAP": ps(float(cur + 1))}, T0)
    info = apply_snapshot(net, snap)
    assert info.changed_taps == 1 and int(net.get_ratio_tap_changers().loc[tid, "tap"]) == cur + 1
