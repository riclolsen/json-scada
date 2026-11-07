"""Offline import and validation report.

Usage: python tools/import_model.py <cgmes.zip> [--no-loadflow] [--round-trip]
Prints the validation report as JSON. Exit code 1 when the model is rejected.
"""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from cim_ems.export import round_trip  # noqa: E402
from cim_ems.models.importer import import_cgmes, sha256_file, validate  # noqa: E402


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("package")
    ap.add_argument("--no-loadflow", action="store_true")
    ap.add_argument("--round-trip", action="store_true")
    a = ap.parse_args()
    net = import_cgmes(a.package)
    report = validate(net, run_load_flow=not a.no_loadflow).as_dict()
    out = {"sha256": sha256_file(a.package), "validation": report}
    if a.round_trip:
        out["roundTrip"] = round_trip(net, boundary_package=a.package)
    print(json.dumps(out, indent=2))
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
