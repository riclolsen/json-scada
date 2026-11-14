"""Svedala demo field simulator: `python -m cim_ems.demo_field [configFile] [--period 5] [--connection 91]`.

Plays the substations of the demo model for the SVEDALA_FIELD connection, like a protocol driver would:
- solves its own copy of the active model under a slow load curve with per-load noise (AC load flow) and writes
  noisy V/P/Q measurements and the switch states to the `sourceDataUpdate` of the field points;
- executes breaker commands from `commandsQueue` (value 1 closes, 0 opens), republishes at once and acknowledges.

Points are found by their address (`protocolSourceCommonAddress` switch/analog, `protocolSourceObjectAddress`
switch id or `<element>|<side>|<quantity>`), so the simulator works with any model and connection set up the same
way. Demo only: no redundancy, the process just writes while it runs.
"""
from __future__ import annotations

import argparse
import logging
import math
import time
from datetime import datetime, timedelta, timezone

import numpy as np
import pypowsybl.loadflow as lf
from pymongo import UpdateOne
from pymongo.errors import PyMongoError

from . import demo, mongo
from .config import DEFAULT_CONFIG, load_main_config
from .models import store as mstore

log = logging.getLogger("CIM-EMS.demo_field")

LOAD_SWING, LOAD_PERIOD_S, LOAD_NOISE = 0.08, 900.0, 0.01   # +-8 % over 15 min, 1 % per load and step
NOISE_OF_SIGMA = 0.3                                         # measurement noise as a fraction of the binding sigma
SWITCH_REFRESH_S = 120.0                                     # integrity republication of all switch states
COMMAND_POLL_S = 0.5
ORIGINATOR = "cim_ems.demo_field"


def lf_params() -> lf.Parameters:
    return lf.Parameters(distributed_slack=True,
                         balance_type=lf.BalanceType.PROPORTIONAL_TO_GENERATION_P_MAX)


def source_update(value: float | None, now: datetime, digital: bool, spontaneous: bool) -> dict:
    valid = value is not None
    return {
        "valueAtSource": float(value) if valid else 0.0,
        "valueStringAtSource": "",
        "asduAtSource": "digital" if digital else "analog",
        "causeOfTransmissionAtSource": "3" if spontaneous else "20",
        "timeTag": now, "timeTagAtSource": now, "timeTagAtSourceOk": True,
        "invalidAtSource": not valid, "substitutedAtSource": False, "overflowAtSource": False,
        "blockedAtSource": False, "notTopicalAtSource": False, "test": False, "originator": ORIGINATOR,
    }


class FieldSimulator:
    def __init__(self, db, net, connection: int, seed: int = 1):
        self.db, self.net, self.connection = db, net, float(connection)
        self.rng = np.random.default_rng(seed)
        self.t0 = time.monotonic()
        loads = net.get_loads()
        self.base_p, self.base_q = loads.p0.copy(), loads.q0.copy()
        self.switches: dict[str, float] = {}               # switch id -> point key
        self.analogs: list[tuple[float, str, int, str]] = []   # key, element, side, quantity
        self.commands: dict[float, str] = {}               # command key -> switch id
        vls = net.get_voltage_levels()["nominal_v"]
        self.nominal = {}
        for getter in (net.get_busbar_sections, net.get_bus_breaker_view_buses):
            for bid, r in getter().iterrows():
                self.nominal.setdefault(bid, float(vls[r.voltage_level_id]))
        self.load_points()
        self.last_switch_refresh = 0.0
        self.published_switches: dict[str, bool] = {}
        self.started = datetime.now(timezone.utc)

    def load_points(self) -> None:
        sw_ids = set(self.net.get_switches().index)
        for d in self.db["realtimeData"].find({"protocolSourceConnectionNumber": self.connection},
                                              {"origin": 1, "protocolSourceCommonAddress": 1,
                                               "protocolSourceObjectAddress": 1}):
            kind, addr = d.get("protocolSourceCommonAddress"), str(d.get("protocolSourceObjectAddress", ""))
            if d.get("origin") == "command" and addr in sw_ids:
                self.commands[float(d["_id"])] = addr
            elif kind == "switch" and addr in sw_ids:
                self.switches[addr] = float(d["_id"])
            elif kind == "analog" and addr.count("|") == 2:
                eid, side, q = addr.split("|")
                self.analogs.append((float(d["_id"]), eid, int(side), q))
        log.info("connection %d: %d switches, %d analogs, %d commands", int(self.connection), len(self.switches),
                 len(self.analogs), len(self.commands))

    # ------------------------------------------------------------ process
    def apply_load_curve(self) -> float:
        t = time.monotonic() - self.t0
        factor = 1.0 + LOAD_SWING * math.sin(2 * math.pi * t / LOAD_PERIOD_S)
        noise = 1.0 + self.rng.normal(0.0, LOAD_NOISE, len(self.base_p))
        self.net.update_loads(id=list(self.base_p.index), p0=list(self.base_p.values * factor * noise),
                              q0=list(self.base_q.values * factor * noise))
        return factor

    def solve(self) -> bool:
        try:
            res = lf.run_ac(self.net, lf_params())
        except Exception as e:                         # an impossible topology must not stop the demo
            log.warning("load flow failed: %s", e)
            return False
        return bool(res) and res[0].status.name == "CONVERGED"

    def step(self) -> dict:
        factor = self.apply_load_curve()
        ok = self.solve()
        now = datetime.now(timezone.utc)
        meas = demo.Measurer(self.net)
        ops = []
        for key, eid, side, q in self.analogs:
            v = meas.value(eid, side, q) if ok else None
            if v is not None:
                sigma = demo.SIGMA_V_REL * self.nominal.get(eid, abs(v)) if q == "V" else demo.SIGMA_PQ
                v += float(self.rng.normal(0.0, sigma * NOISE_OF_SIGMA))
            ops.append(UpdateOne({"_id": key}, {"$set": {"sourceDataUpdate": source_update(v, now, False, False)}}))
        ops += self.switch_ops(now, force=time.monotonic() - self.last_switch_refresh > SWITCH_REFRESH_S)
        if ops:
            self.db["realtimeData"].bulk_write(ops, ordered=False)
        return {"converged": ok, "load": round(factor, 3), "written": len(ops)}

    def switch_ops(self, now: datetime, force: bool = False) -> list:
        if force:
            self.last_switch_refresh = time.monotonic()
        state = self.net.get_switches()["open"]
        ops = []
        for sid, key in self.switches.items():
            is_open = bool(state[sid])
            if force or self.published_switches.get(sid) != is_open:
                spontaneous = sid in self.published_switches and self.published_switches[sid] != is_open
                ops.append(UpdateOne({"_id": key}, {"$set": {
                    "sourceDataUpdate": source_update(0.0 if is_open else 1.0, now, True, spontaneous)}}))
                self.published_switches[sid] = is_open
        return ops

    # ------------------------------------------------------------ commands
    def poll_commands(self) -> int:
        """Execute and acknowledge pending commands of the connection. Returns how many were executed."""
        since = self.started - timedelta(seconds=10)    # never replay old commands after a restart
        done = 0
        for cmd in self.db["commandsQueue"].find({"protocolSourceConnectionNumber": self.connection,
                                                  "ack": {"$exists": False}, "timeTag": {"$gte": since}}):
            sid = self.commands.get(float(cmd.get("pointKey", -1)))
            ok = sid is not None
            if ok:
                close = float(cmd.get("value", 0)) >= 0.5
                self.net.update_switches(id=sid, open=not close)
                log.info("command %s: %s", cmd.get("tag"), "close" if close else "open")
                done += 1
            else:
                log.warning("command %s: unknown point key %s", cmd.get("tag"), cmd.get("pointKey"))
            self.db["commandsQueue"].update_one({"_id": cmd["_id"]}, {"$set": {
                "ack": ok, "ackTimeTag": datetime.now(timezone.utc),
                **({} if ok else {"resultDescription": "unknown command point"})}})
        if done:     # the breaker moved: show the new state and the new flows now
            ops = self.switch_ops(datetime.now(timezone.utc))
            if ops:
                self.db["realtimeData"].bulk_write(ops, ordered=False)
            self.step()
        return done


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("config", nargs="?", default=DEFAULT_CONFIG)
    ap.add_argument("--model", default=demo.MODEL_ID)
    ap.add_argument("--connection", type=int, default=demo.FIELD_CONNECTION[0])
    ap.add_argument("--period", type=float, default=5.0, help="seconds between measurement updates")
    ap.add_argument("--log-level", type=int, default=1)
    a = ap.parse_args(argv)
    logging.basicConfig(level=logging.DEBUG if a.log_level >= 3 else logging.INFO,
                        format="%(asctime)s %(levelname)s %(message)s")
    cfg = load_main_config(a.config)
    while True:
        try:
            db = mongo.connect(cfg)
            model = mstore.get_active(db, a.model)
            if model is not None:
                break
            log.error("model %s has no active revision (demo database not loaded?), retrying", a.model)
        except PyMongoError as e:
            log.error("MongoDB not available (%s), retrying", str(e).split(",")[0])
        time.sleep(10)
    sim = FieldSimulator(db, mstore.load_network(db, model), a.connection)
    log.info("simulating model %s rev %d, period %.1f s", model["modelId"], model["revision"], a.period)
    next_step = 0.0
    while True:
        try:
            sim.poll_commands()
            if time.monotonic() >= next_step:
                next_step = time.monotonic() + a.period
                log.debug("step %s", sim.step())
        except PyMongoError as e:
            log.error("MongoDB: %s", e)
            time.sleep(5)
        time.sleep(COMMAND_POLL_S)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(0)
