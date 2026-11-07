"""Measurement/switch/tap bindings between CIM elements and JSON-SCADA points."""
from __future__ import annotations

import csv
from dataclasses import dataclass, fields
from pathlib import Path

KINDS = ("switch", "tap", "analog")
QUANTITIES = ("V", "P", "Q", "I")


@dataclass
class Binding:
    kind: str                      # switch | tap | analog
    element_id: str                # IIDM/CGMES mRID of the switch, transformer or branch/bus
    tag: str                       # JSON-SCADA tag name
    quantity: str = ""             # analog only: V, P, Q, I
    side: int = 0                  # analog branch end 1/2/3, 0 for buses
    point_key: float | None = None
    closed_value: float = 1.0      # switch: point value meaning closed (processed value, kconv applied)
    sign: float = 1.0              # analog: SCADA to CGMES terminal convention
    sigma: float = 1.0             # analog: standard deviation, same unit as the value
    max_age_seconds: float = 30.0  # analog
    approved: bool = False
    confidence: float = 0.0
    proposed_from: str = "manual"  # cgmes-measurement | name-match | manual

    def validate(self) -> None:
        if self.kind not in KINDS:
            raise ValueError(f"unknown binding kind {self.kind!r}")
        if self.kind == "analog" and self.quantity not in QUANTITIES:
            raise ValueError(f"analog binding needs quantity in {QUANTITIES}, got {self.quantity!r}")
        if self.sigma <= 0:
            raise ValueError("sigma must be positive")


_FLOAT = {"point_key", "closed_value", "sign", "sigma", "max_age_seconds", "confidence"}
_BOOL = {"approved"}
_INT = {"side"}


def write_csv(bindings: list[Binding], path: str | Path) -> None:
    names = [f.name for f in fields(Binding)]
    with open(path, "w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=names)
        w.writeheader()
        for b in bindings:
            w.writerow({n: getattr(b, n) for n in names})


def read_csv(path: str | Path) -> list[Binding]:
    out = []
    with open(path, newline="", encoding="utf-8") as f:
        for row in csv.DictReader(f):
            kw = {}
            for k, v in row.items():
                if k in _BOOL:
                    kw[k] = v.strip().lower() in ("1", "true", "yes", "y")
                elif k in _INT:
                    kw[k] = int(float(v)) if v != "" else 0
                elif k in _FLOAT:
                    kw[k] = float(v) if v != "" else None if k == "point_key" else float(v or 0)
                else:
                    kw[k] = v
            b = Binding(**kw)
            b.validate()
            out.append(b)
    return out
