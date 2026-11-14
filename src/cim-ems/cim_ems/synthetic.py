"""Synthetic JSON-SCADA tags and switching events for a model (tests, demos, replay smoke runs)."""
from __future__ import annotations

import random
from datetime import datetime, timedelta, timezone

import pypowsybl.network as pn

from .bindings.proposal import switch_parts
from .snapshot import PointState

T0 = datetime(2026, 1, 1, 12, 0, 0, tzinfo=timezone.utc)


def switch_tags(net: pn.Network, suffix: str = "") -> list[dict]:
    """One digital tag per switch, named `<substation>;<voltage level>;<switch>`."""
    tags = []
    for i, (sid, (sub, vl, name)) in enumerate(switch_parts(net).items()):
        tags.append({"_id": float(i + 1), "tag": f"{sub};{vl};{name}{suffix}", "description": f"{sub} {vl} {name}",
                     "type": "digital"})
    return tags


def initial_points(net: pn.Network, bindings, closed_value: float = 1.0) -> dict[str, PointState]:
    sw = net.get_switches()
    pts = {}
    for b in bindings:
        if b.kind == "switch":
            pts[b.tag] = PointState(closed_value if not sw.loc[b.element_id, "open"] else 1 - closed_value,
                                    time_tag=T0, time_tag_at_source=T0)
    return pts


def random_events(bindings, n: int, seed: int = 1) -> list[tuple[datetime, str, float]]:
    """Alternate open/close commands on random bound breakers, one every 10 s."""
    rng = random.Random(seed)
    tags = [b for b in bindings if b.kind == "switch"]
    state = {b.tag: b.closed_value for b in tags}
    events = []
    for k in range(n):
        b = rng.choice(tags)
        state[b.tag] = 1 - state[b.tag]
        events.append((T0 + timedelta(seconds=10 * (k + 1)), b.tag, state[b.tag]))
    return events


def analog_tags(net: pn.Network, style: str = "letter", start_key: int = 100000) -> tuple[list[dict], dict[str, tuple]]:
    """Analog tags `<label parts joined by ';'>;<quantity>` for every analog label, and the truth map tag -> key.

    style "letter": quantity suffix P/Q/V; style "unit": suffix MW/MVAR/KV and the unit field set.
    """
    from .bindings.proposal import analog_labels
    words = {"letter": {"P": "P", "Q": "Q", "V": "V"}, "unit": {"P": "MW", "Q": "MVAR", "V": "KV"}}[style]
    units = {"P": "MW", "Q": "Mvar", "V": "kV"}
    tags, truth = [], {}
    labels = analog_labels(net)
    parts = {}
    # labels are space-joined names; rebuild them from the names so spaces inside names survive
    for key, label in labels.items():
        parts[key] = label
    for i, (key, label) in enumerate(sorted(parts.items(), key=lambda kv: str(kv[0]))):
        name = ";".join(label.split(" ")) + ";" + words[key[2]] + (f";{key[1]}" if False else "")
        if name in truth:          # same label on both ends of a branch inside one voltage level: keep the first
            continue
        tags.append({"_id": float(start_key + i), "tag": name, "description": label, "type": "analog",
                     "unit": units[key[2]] if style == "unit" else ""})
        truth[name] = key
    return tags, truth
