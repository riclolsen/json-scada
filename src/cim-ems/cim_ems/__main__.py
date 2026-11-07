"""Driver entry point: `python -m cim_ems <instance> <logLevel> <configFile>`.

One process serves all enabled CIM-EMS connections of the instance. Only the active node computes and
publishes; the change-stream caches keep running on every node.
"""
import logging
import sys
import time

from pymongo.errors import PyMongoError

from . import mongo
from .bindings import store as bstore
from .cache import PointCache
from .config import load_main_config, parse_args
from .cycle import StudyCycle
from .models import store as mstore
from .publish import ResultPublisher
from .redundancy import DRIVER_NAME, Redundancy

LOG_LEVELS = {0: logging.ERROR, 1: logging.INFO, 2: logging.INFO, 3: logging.DEBUG}
MIN_RUN_INTERVAL = 1.0           # debounce for topology-triggered runs, seconds
RELOAD_CHECK_SECONDS = 10.0
MONGO_RETRY_SECONDS = 10


def build_cycle(db, conn: dict, node: str, red: Redundancy, log) -> tuple[StudyCycle, PointCache] | None:
    model = mstore.get_active(db, conn["cimModelId"])
    if model is None:
        log.error("%s: model %s has no active revision", conn["name"], conn["cimModelId"])
        return None
    wanted = conn.get("cimModelRevision")
    if wanted and model["revision"] != wanted:
        model = db[mongo.MODELS].find_one({"modelId": conn["cimModelId"], "revision": wanted})
        if model is None or model["status"] not in ("active", "retired", "validated"):
            log.error("%s: model revision %s not usable", conn["name"], wanted)
            return None
    rev = conn.get("cimBindingRevision") or bstore.latest_revision(db, conn["cimModelId"])
    bindings = bstore.load_revision(db, conn["cimModelId"], rev) if rev else []
    bindings, own = bstore.drop_own_points(db, bindings)
    if own:
        log.warning("%s: %d binding(s) refer to CIM-EMS result points and are ignored, e.g. %s",
                    conn["name"], len(own), own[0].tag)
    conn = {**conn, "cimBindingRevision": rev}
    net = mstore.load_network(db, model)
    tags = [b.tag for b in bindings]
    cache = PointCache(db, tags, {b.tag for b in bindings if b.kind in ("switch", "tap")})
    pub = ResultPublisher(db, red, int(conn["protocolConnectionNumber"]), conn["name"],
                          float(conn.get("resultMaxAgeSeconds", 180)), bool(conn.get("autoCreateTags", True)))
    cycle = StudyCycle(db, conn, model, net, bindings, cache, pub, red, node)
    log.info("%s: model %s rev %s, %d approved bindings", conn["name"], model["modelId"], model["revision"], len(bindings))
    return cycle, cache


def main(argv: list[str]) -> int:
    instance, log_level, config_file = parse_args(argv)
    logging.basicConfig(level=LOG_LEVELS.get(log_level, logging.INFO), format="%(asctime)s %(levelname)s %(message)s")
    log = logging.getLogger(DRIVER_NAME)
    cfg = load_main_config(config_file)
    while True:      # wait for MongoDB like the other drivers instead of exiting with a traceback
        try:
            db = mongo.connect(cfg)
            mongo.ensure_indexes(db)
            break
        except PyMongoError as e:
            log.error("MongoDB not available (%s), retrying in %d s", str(e).split(",")[0], MONGO_RETRY_SECONDS)
            time.sleep(MONGO_RETRY_SECONDS)

    red = Redundancy(db, cfg.node_name, instance)
    doc = red.instance_doc()
    if doc is None or not doc.get("enabled"):
        log.error("%s instance %d missing or disabled, exiting", DRIVER_NAME, instance)
        return 1
    log.info("%s instance %d starting on node %s", DRIVER_NAME, instance, cfg.node_name)

    def connections() -> list[dict]:
        return list(db["protocolConnections"].find(
            {"protocolDriver": DRIVER_NAME, "protocolDriverInstanceNumber": instance, "enabled": True}))

    running: dict[int, dict] = {}     # connection number -> {conn, cycle, cache, next_due, last_run}
    was_active, last_reload = False, 0.0
    while True:
        active = red.tick()
        if active != was_active:
            log.info("node %s is now %s", cfg.node_name, "ACTIVE" if active else "inactive")
            was_active = active

        if time.monotonic() - last_reload > RELOAD_CHECK_SECONDS:
            last_reload = time.monotonic()
            current = {int(c["protocolConnectionNumber"]): c for c in connections()}
            for num in list(running):
                if num not in current or {k: v for k, v in current[num].items() if k != "_id"} != running[num]["conn"]:
                    running.pop(num)["cache"].stop()
            for num, conn in current.items():
                if num in running:
                    continue
                built = build_cycle(db, conn, cfg.node_name, red, log)
                if built:
                    cycle, cache = built
                    cache.start()
                    running[num] = {"conn": {k: v for k, v in conn.items() if k != "_id"}, "cycle": cycle,
                                    "cache": cache, "next_due": 0.0, "last_run": 0.0}

        if active:
            now = time.monotonic()
            for r in running.values():
                if not r["cache"].ready.is_set():
                    continue
                first = not r.get("started")
                triggered = r["cache"].trigger.is_set() and now - r["last_run"] >= MIN_RUN_INTERVAL
                if first or triggered or now >= r["next_due"]:
                    r["cache"].trigger.clear()
                    r["started"] = True
                    r["cycle"].run_once("start" if first else "topology" if triggered else "cycle")
                    r["last_run"] = time.monotonic()
                    r["next_due"] = r["last_run"] + float(r["conn"].get("cycleSeconds", 60))
        time.sleep(0.5)


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except KeyboardInterrupt:
        sys.exit(0)
