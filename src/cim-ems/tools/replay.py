"""Replay switching events through the snapshot policy and topology processor.

  python tools/replay.py <model.zip> <bindings.csv> <events.csv>
  python tools/replay.py <model.zip> --synthetic [N]     (propose from synthetic tags, N random events)

events.csv columns: time (ISO 8601), tag, value[, invalid, transient]
Prints one line per event: topology hash, buses, components, flags, load flow status.
"""
import argparse
import csv
import sys
from datetime import datetime
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pypowsybl.loadflow as lf  # noqa: E402

from cim_ems import synthetic  # noqa: E402
from cim_ems.bindings import model as bmodel  # noqa: E402
from cim_ems.bindings.proposal import propose_switches  # noqa: E402
from cim_ems.models.importer import import_cgmes  # noqa: E402
from cim_ems.snapshot import PointState, SnapshotBuilder  # noqa: E402
from cim_ems.topology import apply_snapshot  # noqa: E402


def replay(net, bindings, events, run_load_flow=True):
    """events: iterable of (time, tag, value, invalid, transient). Returns a list of result dicts."""
    builder = SnapshotBuilder(bindings)
    points = synthetic.initial_points(net, bindings)
    t = synthetic.T0
    builder.build(points, t)
    apply_snapshot(net, builder.build(points, t))
    rows = []
    for time, tag, value, invalid, transient in events:
        points[tag] = PointState(value, invalid=invalid, transient=transient, time_tag=time, time_tag_at_source=time)
        snap = builder.build(points, time)
        info = apply_snapshot(net, snap)
        status = ""
        if run_load_flow and not snap.rejected:
            status = lf.run_ac(net)[0].status.name
        rows.append({"time": time, "tag": tag, "hash": info.hash, "buses": info.buses,
                     "components": info.connected_components, "isolated": info.isolated_buses,
                     "open": info.switches_open, "flags": sorted(snap.flags), "rejected": snap.rejected,
                     "reasons": snap.reasons, "loadflow": status})
    return rows


def read_events(path):
    with open(path, newline="") as f:
        for r in csv.DictReader(f):
            yield (datetime.fromisoformat(r["time"]), r["tag"], float(r["value"]),
                   r.get("invalid", "").lower() in ("1", "true"), r.get("transient", "").lower() in ("1", "true"))


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("model")
    ap.add_argument("bindings", nargs="?")
    ap.add_argument("events", nargs="?")
    ap.add_argument("--synthetic", nargs="?", const=10, type=int)
    a = ap.parse_args()
    net = import_cgmes(a.model)
    if a.synthetic is not None:
        prop = propose_switches(net, synthetic.switch_tags(net))
        bindings = prop.bindings
        for b in bindings:
            b.approved = True
        events = [(t, tag, v, False, False) for t, tag, v in synthetic.random_events(bindings, a.synthetic)]
        print(f"{len(bindings)} switch bindings proposed, {len(prop.unmatched)} unmatched, {len(prop.ambiguous)} ambiguous")
    else:
        bindings = [b for b in bmodel.read_csv(a.bindings)]
        events = list(read_events(a.events))
    for r in replay(net, bindings, events):
        print(r["time"].isoformat(), r["hash"], f"buses={r['buses']}", f"cc={r['components']}", f"iso={r['isolated']}",
              f"open={r['open']}", r["loadflow"] or ("REJECTED" if r["rejected"] else "-"), ",".join(r["flags"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
