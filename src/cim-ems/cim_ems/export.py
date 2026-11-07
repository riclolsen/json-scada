"""CGMES 2.4.15 export and the round-trip check."""
from __future__ import annotations

import re
import tempfile
import zipfile
from pathlib import Path

import pypowsybl.loadflow as lf
import pypowsybl.network as pn

from .models.importer import import_cgmes

EXPORT_PARAMETERS = {
    "iidm.export.cgmes.cim-version": "16",  # CGMES 2.4.15
    "iidm.export.cgmes.profiles": "EQ,SSH,SV,TP",
}


def export_cgmes(net: pn.Network, path: str | Path, profiles: str = "EQ,SSH,SV,TP") -> None:
    net.save(str(path), format="CGMES", parameters={**EXPORT_PARAMETERS, "iidm.export.cgmes.profiles": profiles})


BOUNDARY_FILE = re.compile(r"(_BD[_.]|boundary)", re.IGNORECASE)


def _add_boundary(exported: Path, source_package: Path) -> None:
    """Copy the boundary set (EQ_BD, TP_BD) of the source package into the exported zip."""
    with zipfile.ZipFile(source_package) as src, zipfile.ZipFile(exported, "a", zipfile.ZIP_DEFLATED) as dst:
        for name in src.namelist():
            if BOUNDARY_FILE.search(name) and name not in dst.namelist():
                dst.writestr(name, src.read(name))


def round_trip(net: pn.Network, tol_mw: float = 0.5, boundary_package: str | Path | None = None) -> dict:
    """Export, re-import, solve, and compare identity, topology and solved values.

    The plan compares solved values, never XML bytes. Q mismatches are reported
    per element, so unsupported classes (VSC converters) show up.
    """
    with tempfile.TemporaryDirectory() as tmp:
        f = Path(tmp) / "rt.zip"
        export_cgmes(net, f)
        if boundary_package is not None:
            _add_boundary(f, Path(boundary_package))
        net2 = import_cgmes(f)
    lf.run_ac(net)
    lf.run_ac(net2)

    problems: list[str] = []
    for name in ("get_lines", "get_2_windings_transformers", "get_switches", "get_generators", "get_loads"):
        a, b = getattr(net, name)(), getattr(net2, name)()
        missing = set(a.index) - set(b.index)
        if missing:
            problems.append(f"{name}: {len(missing)} element(s) lost, e.g. {sorted(missing)[:3]}")
    sw_a, sw_b = net.get_switches(), net2.get_switches()
    common = sw_a.index.intersection(sw_b.index)
    diff_sw = (sw_a.loc[common, "open"] != sw_b.loc[common, "open"]).sum()
    if diff_sw:
        problems.append(f"{diff_sw} switch state(s) differ")

    max_dp = max_dq = 0.0
    for name in ("get_lines", "get_2_windings_transformers"):
        a, b = getattr(net, name)(), getattr(net2, name)()
        c = a.index.intersection(b.index)
        for side in ("1", "2"):
            for col, kind in (("p", "p"), ("q", "q")):
                d = (a.loc[c, f"{col}{side}"] - b.loc[c, f"{col}{side}"]).abs().dropna()
                if len(d):
                    if kind == "p":
                        max_dp = max(max_dp, float(d.max()))
                    else:
                        max_dq = max(max_dq, float(d.max()))
    if max_dp > tol_mw:
        problems.append(f"max active power difference {max_dp:.3f} MW")
    if max_dq > tol_mw:
        problems.append(f"max reactive power difference {max_dq:.3f} Mvar")
    return {"ok": not problems, "problems": problems, "max_dp_mw": max_dp, "max_dq_mvar": max_dq}
