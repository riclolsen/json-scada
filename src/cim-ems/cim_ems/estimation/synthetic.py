"""Synthetic measurement sets from a solved pypowsybl network, for tests and tuning."""
from __future__ import annotations

import numpy as np
import pypowsybl.network as pn

from .adapter import Measurement
from .transfer import Transfer


def from_load_flow(src: pn.Network, tr: Transfer, rng: np.random.Generator | None = None, noise: bool = True,
                   sigma_v_kv_rel: float = 0.002, sigma_pq_mva: float = 0.5) -> list[Measurement]:
    rng = rng or np.random.default_rng(1)
    out: list[Measurement] = []

    def mk(eid, q, side, true, sigma):
        v = true + (rng.normal(0, sigma) if noise else 0.0)
        out.append(Measurement(eid, q, side, float(v), float(sigma), tag=f"{eid}.{q}{side}"))

    buses = src.get_buses()
    for bid in tr.bus_index:
        v = float(buses.loc[bid, "v_mag"])
        mk(bid, "V", 0, v, v * sigma_v_kv_rel)
    for table, ids in ((src.get_lines(), tr.line_index), (src.get_2_windings_transformers(), tr.trafo_index)):
        for eid in ids:
            r = table.loc[eid]
            for side in (1, 2):
                for q, col in (("P", f"p{side}"), ("Q", f"q{side}")):
                    mk(eid, q, side, float(r[col]), sigma_pq_mva)
    return out
