"""Generate svg-scada single-line displays.

  python tools/gen_displays.py <model.zip> <bindings.csv> --connection CIM-EMS1 --out svg/cim [--vl ID ...]

Runs one load flow so the result tags (and bus ids) exist, then writes one SVG per voltage level.
Only approved switch bindings are bound; unbound switches are listed.
"""
import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pypowsybl.loadflow as lf  # noqa: E402

from cim_ems.bindings import model as bmodel  # noqa: E402
from cim_ems.diagrams.generate import generate_all  # noqa: E402
from cim_ems.models.importer import import_cgmes  # noqa: E402
from cim_ems.results import extract_loadflow  # noqa: E402


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("model")
    ap.add_argument("bindings")
    ap.add_argument("--connection", required=True, help="protocol connection name (prefix of the result tags)")
    ap.add_argument("--out", required=True)
    ap.add_argument("--vl", nargs="*")
    a = ap.parse_args(argv)
    net = import_cgmes(a.model)
    lf.run_ac(net)
    values = extract_loadflow(net)
    reps = generate_all(net, a.connection, bmodel.read_csv(a.bindings), values, a.out, a.vl)
    for r in reps:
        print(f"{r.voltage_level}: switches {r.switches_bound} bound, {len(r.switches_unbound)} unbound; "
              f"values {r.values_bound}; busbars {r.busbars_bound}")


if __name__ == "__main__":
    main()
