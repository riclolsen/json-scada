"""CGMES 2.4.15 import with pinned parameters, and the validation report."""
from __future__ import annotations

import hashlib
from dataclasses import dataclass, field, asdict
from pathlib import Path

import pypowsybl.loadflow as lf
import pypowsybl.network as pn

# Pinned import parameters. Recorded in cimModels.engine.importParameters.
IMPORT_PARAMETERS = {
    "iidm.import.cgmes.convert-boundary": "false",
    "iidm.import.cgmes.import-node-breaker-as-bus-breaker": "false",
}

# Equipment classes the plan flags per engine (see CIM_EMS_PLAN.md section 20).
APPROXIMATED = {
    "hvdc_lines": "HVDC line: reactive setpoint of VSC converters is not preserved by CGMES 2.4.15 export",
    "vsc_converter_stations": "VSC converter station: reactive setpoint lost on CGMES round trip",
    "lcc_converter_stations": "LCC converter station (classic HVDC): not solved by the AC load flow, no calculation when it connects the only generation",
    "dangling_lines": "Dangling line (boundary): requires boundary set or equivalent injection",
    "three_windings_transformers": "3-winding transformer: not supported by the pandapower estimator path",
}


@dataclass
class ValidationReport:
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    unsupported: list[str] = field(default_factory=list)
    counts: dict[str, int] = field(default_factory=dict)
    islands: int = 0
    load_flow: dict = field(default_factory=dict)

    @property
    def ok(self) -> bool:
        return not self.errors

    def as_dict(self) -> dict:
        d = asdict(self)
        d["ok"] = self.ok
        return d


def sha256_file(path: str | Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


CIM_NAMESPACES = {
    b"http://iec.ch/TC57/2013/CIM-schema-cim16#": "2.4.15",
    b"http://iec.ch/TC57/CIM100#": "3.0",
}


def detect_cgmes_version(path: str | Path) -> str:
    """CGMES version from the CIM namespace of the package files ("unknown" when none is found)."""
    import zipfile

    def sniff(head: bytes) -> str | None:
        for ns, version in CIM_NAMESPACES.items():
            if ns in head:
                return version
        return None

    path = Path(path)
    if zipfile.is_zipfile(path):
        with zipfile.ZipFile(path) as z:
            for name in z.namelist():
                if name.lower().endswith(".xml"):
                    with z.open(name) as f:
                        version = sniff(f.read(4096))
                    if version:
                        return version
        return "unknown"
    with open(path, "rb") as f:
        return sniff(f.read(4096)) or "unknown"


def import_cgmes(path: str | Path, extra: dict | None = None) -> pn.Network:
    params = {**IMPORT_PARAMETERS, **(extra or {})}
    return pn.load(str(path), parameters=params)


def _tables(net: pn.Network) -> dict[str, "object"]:
    return {
        "substations": net.get_substations(),
        "voltage_levels": net.get_voltage_levels(),
        "lines": net.get_lines(),
        "two_windings_transformers": net.get_2_windings_transformers(),
        "three_windings_transformers": net.get_3_windings_transformers(),
        "switches": net.get_switches(),
        "generators": net.get_generators(),
        "loads": net.get_loads(),
        "shunt_compensators": net.get_shunt_compensators(),
        "static_var_compensators": net.get_static_var_compensators(),
        "dangling_lines": net.get_boundary_lines(),
        "hvdc_lines": net.get_hvdc_lines(),
        "vsc_converter_stations": net.get_vsc_converter_stations(),
        "lcc_converter_stations": net.get_lcc_converter_stations(),
    }


def validate(net: pn.Network, run_load_flow: bool = True) -> ValidationReport:
    r = ValidationReport()
    tables = _tables(net)
    r.counts = {k: len(v) for k, v in tables.items()}

    for cls, msg in APPROXIMATED.items():
        if r.counts.get(cls):
            r.unsupported.append(f"{msg} ({r.counts[cls]})")

    if not r.counts["voltage_levels"]:
        r.errors.append("model has no voltage levels")
    if not r.counts["generators"] and not r.counts["dangling_lines"]:
        r.warnings.append("no generator or boundary injection: no slack candidate")

    lines = tables["lines"]
    if len(lines):
        bad = lines[(lines.x == 0) & (lines.r == 0)]
        if len(bad):
            r.errors.append(f"{len(bad)} line(s) with zero impedance: {', '.join(bad.index[:5])}")
    t2 = tables["two_windings_transformers"]
    if len(t2):
        bad = t2[(t2.x == 0) & (t2.r == 0)]
        if len(bad):
            r.errors.append(f"{len(bad)} transformer(s) with zero impedance: {', '.join(bad.index[:5])}")

    buses = net.get_buses()
    r.islands = int(buses.synchronous_component.nunique()) if len(buses) and "synchronous_component" in buses else 0
    if r.islands > 1:
        r.warnings.append(f"{r.islands} synchronous components")

    if run_load_flow and r.ok:
        results = lf.run_ac(net)
        if not results:
            r.errors.append("no network component can be calculated (no buses or no slack candidate)")
            return r
        main = results[0]
        r.load_flow = {
            "status": str(main.status.name),
            "iterations": int(main.iteration_count),
            "components": len(results),
        }
        if main.status.name != "CONVERGED":
            r.errors.append(f"base case load flow {main.status.name}")
    return r
