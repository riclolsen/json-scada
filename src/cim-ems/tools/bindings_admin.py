"""Binding proposals and revisions.

  python tools/bindings_admin.py propose <package.zip> --out bindings.csv [--tags tags.json] [--report report.json]
  python tools/bindings_admin.py save <modelId> <bindings.csv> [--user NAME]
  python tools/bindings_admin.py export <modelId> [<revision>] --out bindings.csv

propose: switch and analog proposals by name matching. Tags come from `--tags` (a JSON list of realtimeData-like
documents with tag, description, type, unit, _id) or from the database (supervised points, never CIM-EMS results).
Nothing is approved: review the CSV, set `approved` to true, then `save` it as a new revision.
"""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from cim_ems import mongo  # noqa: E402
from cim_ems.bindings import model as bmodel  # noqa: E402
from cim_ems.bindings import store as bstore  # noqa: E402
from cim_ems.bindings.proposal import propose_analogs, propose_switches  # noqa: E402
from cim_ems.config import DEFAULT_CONFIG, load_main_config  # noqa: E402
from cim_ems.models.importer import import_cgmes  # noqa: E402


def propose(package: str, tags: list[dict]) -> tuple[list, dict]:
    net = import_cgmes(package)
    sw, an = propose_switches(net, tags), propose_analogs(net, tags)
    report = {
        "tags": len(tags),
        "switches": {"proposed": len(sw.bindings), "unmatched": sw.unmatched, "ambiguous": sw.ambiguous},
        "analogs": {"proposed": len(an.bindings), "unmatched": an.unmatched, "ambiguous": an.ambiguous},
    }
    return sw.bindings + an.bindings, report


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--config", default=DEFAULT_CONFIG)
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("propose")
    p.add_argument("package")
    p.add_argument("--out", required=True)
    p.add_argument("--tags")
    p.add_argument("--report")
    p = sub.add_parser("save")
    p.add_argument("model_id")
    p.add_argument("csv")
    p.add_argument("--user", default="cli")
    p = sub.add_parser("export")
    p.add_argument("model_id")
    p.add_argument("revision", nargs="?", type=int)
    p.add_argument("--out", required=True)
    a = ap.parse_args(argv)

    db = None
    if a.cmd != "propose" or not a.tags:
        db = mongo.connect(load_main_config(a.config))
    if a.cmd == "propose":
        tags = json.loads(Path(a.tags).read_text(encoding="utf-8")) if a.tags else bstore.candidate_tags(db)
        bindings, report = propose(a.package, tags)
        bmodel.write_csv(bindings, a.out)
        if a.report:
            Path(a.report).write_text(json.dumps(report, indent=2), encoding="utf-8")
        print(f"{report['tags']} tags; switches {report['switches']['proposed']} proposed, "
              f"{len(report['switches']['unmatched'])} unmatched, {len(report['switches']['ambiguous'])} ambiguous; "
              f"analogs {report['analogs']['proposed']} proposed, {len(report['analogs']['unmatched'])} unmatched, "
              f"{len(report['analogs']['ambiguous'])} ambiguous")
        return 0
    if a.cmd == "save":
        bindings = bmodel.read_csv(a.csv)
        kept, own = bstore.drop_own_points(db, bindings)
        if own:
            print(f"refused: {len(own)} binding(s) refer to CIM-EMS result points, e.g. {own[0].tag}")
            return 1
        rev = bstore.save_revision(db, a.model_id, kept, a.user)
        print(f"{a.model_id}: binding revision {rev}, {sum(b.approved for b in kept)} of {len(kept)} approved")
        return 0
    rev = a.revision or bstore.latest_revision(db, a.model_id)
    if rev is None:
        print("no binding revision")
        return 1
    bmodel.write_csv(bstore.load_revision(db, a.model_id, rev, approved_only=False), a.out)
    print(f"{a.model_id}: revision {rev} written to {a.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
