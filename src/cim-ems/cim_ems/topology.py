"""Apply a snapshot to the network and describe the resulting topology."""
from __future__ import annotations

import hashlib
from dataclasses import dataclass

import pypowsybl.network as pn

from .snapshot import Snapshot


@dataclass
class TopologyInfo:
    hash: str
    buses: int
    connected_components: int
    isolated_buses: int          # buses outside the largest connected component
    switches_open: int
    changed_switches: int
    changed_taps: int


def topology_hash(net: pn.Network) -> str:
    """Deterministic over switch states and tap positions only."""
    sw = net.get_switches()
    parts = [f"{i}:{int(o)}" for i, o in sorted(sw.open.items())]
    for df in (net.get_ratio_tap_changers(), net.get_phase_tap_changers()):
        parts += [f"{i}:{t}" for i, t in sorted(df.tap.items())] if len(df) else []
    return hashlib.sha256("|".join(parts).encode()).hexdigest()[:12]


def apply_snapshot(net: pn.Network, snap: Snapshot) -> TopologyInfo:
    sw = net.get_switches()
    known = {i: bool(o) for i, o in sw.open.items()}
    unknown = [i for i in snap.switches if i not in known]
    if unknown:
        snap.reasons.append(f"{len(unknown)} bound switch(es) not in the model, e.g. {unknown[:3]}")
        snap.flags.add("bindingMismatch")
    change = {i: o for i, o in snap.switches.items() if i in known and known[i] != o}
    if change:
        net.update_switches(id=list(change), open=list(change.values()))

    tap_changed = 0
    for tid, pos in snap.taps.items():
        for getter, setter in ((net.get_ratio_tap_changers, net.update_ratio_tap_changers),
                               (net.get_phase_tap_changers, net.update_phase_tap_changers)):
            df = getter()
            if tid in df.index:
                if int(df.loc[tid, "tap"]) != pos:
                    setter(id=tid, tap=pos)
                    tap_changed += 1
                break
        else:
            snap.flags.add("bindingMismatch")
            snap.reasons.append(f"tap changer {tid} not in the model")

    buses = net.get_buses()
    cc = buses.connected_component if len(buses) else buses
    sizes = cc.value_counts() if len(buses) else cc
    return TopologyInfo(
        hash=topology_hash(net),
        buses=len(buses),
        connected_components=int(len(sizes)),
        isolated_buses=int(len(buses) - (sizes.max() if len(sizes) else 0)),
        switches_open=int(net.get_switches().open.sum()),
        changed_switches=len(change),
        changed_taps=tap_changed,
    )
