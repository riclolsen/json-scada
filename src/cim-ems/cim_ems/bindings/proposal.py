"""Propose bindings by matching CIM names with JSON-SCADA tag names and descriptions.

Proposals are never approved here: an engineer reviews the CSV (confidence, unmatched
elements, coverage) and sets `approved` before a binding revision is created.
"""
from __future__ import annotations

import re
import unicodedata
from dataclasses import dataclass, field

import pypowsybl.network as pn

from .model import Binding

AUTO_THRESHOLD = 0.75
MIN_MARGIN = 0.10


def tokens(text: str) -> set[str]:
    """Lower-case, accent-free tokens: letter/digit parts ("VLA132KV" -> vla, 132, kv) plus the whole word."""
    text = unicodedata.normalize("NFKD", text or "").encode("ascii", "ignore").decode().lower()
    out: set[str] = set()
    for part in filter(None, re.split(r"[^a-z0-9]+", text)):
        out.update(re.findall(r"[a-z]+|[0-9]+", part))
        out.add("w:" + part)   # whole word, so "breaker132" differs from "breaker339" beyond the shared "132"
    return out


def dice(a: set[str], b: set[str]) -> float:
    return 2 * len(a & b) / (len(a) + len(b)) if a and b else 0.0


@dataclass
class Proposal:
    bindings: list[Binding] = field(default_factory=list)
    unmatched: list[str] = field(default_factory=list)   # element ids without an acceptable tag
    ambiguous: list[str] = field(default_factory=list)   # element ids with close competing tags

    def coverage(self, net: pn.Network) -> dict[str, dict]:
        """Share of switches with a proposed binding, per voltage level."""
        sw = net.get_switches()
        bound = {b.element_id for b in self.bindings if b.kind == "switch"}
        res: dict[str, dict] = {}
        for vl, grp in sw.groupby("voltage_level_id"):
            n = int(grp.index.isin(bound).sum())
            res[vl] = {"switches": len(grp), "bound": n}
        return res


def switch_parts(net: pn.Network) -> dict[str, tuple[str, str, str]]:
    """(substation, voltage level, switch) names per switch id; ids replace empty names."""
    sw = net.get_switches()
    vl = net.get_voltage_levels()[["name", "substation_id"]]
    sub = net.get_substations()["name"]
    out = {}
    for sid, row in sw.iterrows():
        v = vl.loc[row.voltage_level_id]
        out[sid] = (sub.get(v.substation_id) or v.substation_id or "", v["name"] or row.voltage_level_id,
                    row["name"] or sid)
    return out


def switch_labels(net: pn.Network) -> dict[str, str]:
    return {sid: " ".join(parts) for sid, parts in switch_parts(net).items()}


def ascii_lower(text: str) -> str:
    return unicodedata.normalize("NFKD", text or "").encode("ascii", "ignore").decode().lower()


def part_regex(part: str) -> re.Pattern | None:
    """Whole-word pattern for a name; separators in the name may differ in the tag ("A_B" matches "a-b")."""
    runs = re.findall(r"[a-z0-9]+", ascii_lower(part))
    if not runs:
        return None
    return re.compile(r"(?<![a-z0-9])" + r"[^a-z0-9]*".join(map(re.escape, runs)) + r"(?![a-z0-9])")


def _best(label: str, tags: list[tuple[set[str], str, set[str], dict]]) -> tuple[dict | None, float, float]:
    """Dice score on tokens, with an order-aware boost.

    - A tag holding every label part as a whole word, in order, and nothing else scores 1.0.
    - The same with extra tokens in the tag scores 0.85 minus 0.005 per extra token (floor 0.80), so "FT63_G1-S"
      does not tie with "FT63_G1-S(1)" while a tag carrying a harmless suffix still passes the threshold.
    - Anything else is capped at 0.85 of its Dice score, so "bbs1" outranks "bbs2" although they share most tokens.
    """
    lt = tokens(label)
    parts = [r for r in map(part_regex, label.split(" ")) if r]
    def score(tt: set[str], flat: str, own: set[str]) -> float:
        pos, ordered = 0, True
        for r in parts:
            m = r.search(flat, pos)
            if m is None:
                ordered = False
                break
            pos = m.end()
        if not ordered:
            return dice(lt, tt) * 0.85
        extra = len(own - lt)
        return 1.0 if extra == 0 else max(0.80, 0.85 - 0.005 * extra)
    scored = sorted(((score(tt, flat, own), t) for tt, flat, own, t in tags), key=lambda x: -x[0])
    if not scored:
        return None, 0.0, 0.0
    top = scored[0][0]
    second = scored[1][0] if len(scored) > 1 else 0.0
    return scored[0][1], top, second


def _assign(prop: Proposal, best: list, make) -> None:
    """Highest scores first, so a tag goes to the element it matches best; a weaker claimant stays unmatched."""
    used: set[str] = set()
    for top, second, key, tag in sorted(best, key=lambda x: -x[0]):
        if tag is None or top < AUTO_THRESHOLD or tag["tag"] in used:
            prop.unmatched.append(key)
        elif top - second < MIN_MARGIN:
            prop.ambiguous.append(key)
        else:
            used.add(tag["tag"])
            prop.bindings.append(make(key, tag, top))


def propose_switches(net: pn.Network, tags: list[dict]) -> Proposal:
    """`tags`: realtimeData-like dicts with `tag`, `description`, `type`, `_id`. Only digital tags are used."""
    digital = [(tokens(f"{t['tag']} {t.get('description', '')}"), ascii_lower(t["tag"]), tokens(t["tag"]), t)
               for t in tags if t.get("type", "digital") == "digital"]
    prop = Proposal()
    best = []
    for sid, label in switch_labels(net).items():
        tag, top, second = _best(label, digital)
        best.append((top, second, sid, tag))
    _assign(prop, best, lambda sid, tag, top: Binding(
        kind="switch", element_id=sid, tag=tag["tag"], point_key=tag.get("_id"), confidence=round(top, 3),
        proposed_from="name-match"))
    return prop


# --- analog measurements -------------------------------------------------------------------------------------------

UNITS = {"kv": "V", "mw": "P", "mvar": "Q", "mvr": "Q"}
QUANTITY_WORDS = {"p": "P", "mw": "P", "q": "Q", "mvar": "Q", "mvr": "Q", "v": "V", "kv": "V", "u": "V"}
DESCRIPTION_WORDS = (("reactive", "Q"), ("active", "P"), ("voltage", "V"))


def quantity_of(tag: dict) -> str | None:
    """V, P or Q from the unit, else the last quantity word of the tag name, else the description."""
    unit = ascii_lower(tag.get("unit", "")).strip()
    if unit in UNITS:
        return UNITS[unit]
    for word in reversed(re.split(r"[^a-z0-9]+", ascii_lower(tag["tag"]))):
        if word in QUANTITY_WORDS:
            return QUANTITY_WORDS[word]
    desc = ascii_lower(tag.get("description", ""))
    for word, q in DESCRIPTION_WORDS:
        if word in desc:
            return q
    return None


def analog_labels(net: pn.Network) -> dict[tuple[str, int, str], str]:
    """Labels per (element id, side, quantity).

    - Branch ends: "<substation> <branch>", or "<substation> <voltage level> <branch>" when both ends are in the same
      substation (transformers), for P and Q.
    - Voltages: "<substation> <voltage level>" for V, bound to the first busbar section of the voltage level, or to
      its first bus in a bus-breaker model.
    """
    vls = net.get_voltage_levels()[["name", "substation_id"]]
    subs = net.get_substations()["name"]

    def sub_of(vl):
        return subs.get(vls.loc[vl, "substation_id"]) or vls.loc[vl, "substation_id"] or ""

    def vl_name(vl):
        return vls.loc[vl, "name"] or vl

    out: dict[tuple[str, int, str], str] = {}
    for df in (net.get_lines(), net.get_2_windings_transformers()):
        for eid, r in df.iterrows():
            s1, s2 = sub_of(r.voltage_level1_id), sub_of(r.voltage_level2_id)
            name = r["name"] or eid
            for side, vl, sub in ((1, r.voltage_level1_id, s1), (2, r.voltage_level2_id, s2)):
                label = f"{sub} {vl_name(vl)} {name}" if s1 == s2 else f"{sub} {name}"
                for q in ("P", "Q"):
                    out[(eid, side, q)] = label
    bbs = net.get_busbar_sections()
    first: dict[str, str] = {}
    for bid, r in bbs.iterrows():
        first.setdefault(r.voltage_level_id, bid)
    for bid, r in net.get_bus_breaker_view_buses().iterrows():
        first.setdefault(r.voltage_level_id, bid)
    for vl, eid in first.items():
        out[(eid, 0, "V")] = f"{sub_of(vl)} {vl_name(vl)}"
    return out


def propose_analogs(net: pn.Network, tags: list[dict], sigma_v_rel: float = 0.004, sigma_pq: float = 1.0) -> Proposal:
    """`tags`: realtimeData-like dicts (`tag`, `description`, `type`, `unit`, `_id`). Only analog tags with a
    recognizable quantity are used; each tag competes only for elements of its quantity."""
    by_q: dict[str, list] = {"V": [], "P": [], "Q": []}
    for t in tags:
        if t.get("type") != "analog":
            continue
        q = quantity_of(t)
        if q in by_q:
            by_q[q].append((tokens(f"{t['tag']} {t.get('description', '')}"), ascii_lower(t["tag"]), tokens(t["tag"]), t))
    nominal = {}
    vls = net.get_voltage_levels()["nominal_v"]
    for df in (net.get_busbar_sections(), net.get_bus_breaker_view_buses()):
        for eid, r in df.iterrows():
            nominal[eid] = float(vls[r.voltage_level_id])
    prop = Proposal()
    best = []
    labels = analog_labels(net)
    # parallel circuits with the same name (and both ends of a tie line without substation) cannot be told apart
    seen: dict[tuple[str, str], int] = {}
    for (eid, side, q), label in labels.items():
        seen[(q, label)] = seen.get((q, label), 0) + 1
    for key, label in labels.items():
        if seen[(key[2], label)] > 1:
            prop.ambiguous.append(key)
            continue
        tag, top, second = _best(label, by_q[key[2]])
        best.append((top, second, key, tag))

    def make(key, tag, top):
        eid, side, q = key
        sigma = sigma_v_rel * nominal.get(eid, 1.0) if q == "V" else sigma_pq
        return Binding(kind="analog", element_id=eid, tag=tag["tag"], quantity=q, side=side,
                       point_key=tag.get("_id"), sigma=sigma, confidence=round(top, 3), proposed_from="name-match")

    _assign(prop, best, make)
    prop.unmatched = [f"{e}|{s}|{q}" for e, s, q in prop.unmatched]
    prop.ambiguous = [f"{e}|{s}|{q}" for e, s, q in prop.ambiguous]
    return prop
