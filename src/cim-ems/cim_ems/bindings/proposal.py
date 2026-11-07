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


def _best(label: str, tags: list[tuple[set[str], str, dict]]) -> tuple[dict | None, float, float]:
    """Dice score on tokens. A tag that contains every label part as a whole word, in order, scores 1.0, others
    are capped at 0.95 of their Dice score, so "bbs1" outranks "bbs2" although they share most tokens."""
    lt = tokens(label)
    parts = [r for r in map(part_regex, label.split(" ")) if r]
    def score(tt: set[str], flat: str) -> float:
        pos, ordered = 0, True
        for r in parts:
            m = r.search(flat, pos)
            if m is None:
                ordered = False
                break
            pos = m.end()
        return 1.0 if ordered else dice(lt, tt) * 0.95
    scored = sorted(((score(tt, flat), t) for tt, flat, t in tags), key=lambda x: -x[0])
    if not scored:
        return None, 0.0, 0.0
    top = scored[0][0]
    second = scored[1][0] if len(scored) > 1 else 0.0
    return scored[0][1], top, second


def propose_switches(net: pn.Network, tags: list[dict]) -> Proposal:
    """`tags`: realtimeData-like dicts with `tag`, `description`, `type`, `_id`. Only digital tags are used."""
    digital = [(tokens(f"{t['tag']} {t.get('description', '')}"), ascii_lower(t["tag"]), t)
               for t in tags if t.get("type", "digital") == "digital"]
    prop = Proposal()
    best = []
    for sid, label in switch_labels(net).items():
        tag, top, second = _best(label, digital)
        best.append((top, second, sid, tag))
    # Highest scores first, so a tag goes to the element it matches best; a weaker claimant stays unmatched.
    used: set[str] = set()
    for top, second, sid, tag in sorted(best, key=lambda x: -x[0]):
        if tag is None or top < AUTO_THRESHOLD or tag["tag"] in used:
            prop.unmatched.append(sid)
        elif top - second < MIN_MARGIN:
            prop.ambiguous.append(sid)
        else:
            used.add(tag["tag"])
            prop.bindings.append(Binding(kind="switch", element_id=sid, tag=tag["tag"], point_key=tag.get("_id"),
                                         confidence=round(top, 3), proposed_from="name-match"))
    return prop
