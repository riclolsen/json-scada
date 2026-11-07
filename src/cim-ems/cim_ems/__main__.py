"""Driver entry point: `python -m cim_ems <instance> <logLevel> <configFile>`.

Phase 1 scope: configuration, instance check and active-node keep-alive. The
study cycle is added in later phases.
"""
import logging
import sys
import time

from . import mongo
from .config import load_main_config, parse_args
from .redundancy import DRIVER_NAME, Redundancy

LOG_LEVELS = {0: logging.ERROR, 1: logging.INFO, 2: logging.INFO, 3: logging.DEBUG}


def main(argv: list[str]) -> int:
    instance, log_level, config_file = parse_args(argv)
    logging.basicConfig(level=LOG_LEVELS.get(log_level, logging.INFO), format="%(asctime)s %(levelname)s %(message)s")
    log = logging.getLogger(DRIVER_NAME)
    cfg = load_main_config(config_file)
    db = mongo.connect(cfg)
    mongo.ensure_indexes(db)

    red = Redundancy(db, cfg.node_name, instance)
    doc = red.instance_doc()
    if doc is None or not doc.get("enabled"):
        log.error("%s instance %d missing or disabled, exiting", DRIVER_NAME, instance)
        return 1
    log.info("%s instance %d starting on node %s", DRIVER_NAME, instance, cfg.node_name)

    was_active = False
    while True:
        active = red.tick()
        if active != was_active:
            log.info("node %s is now %s", cfg.node_name, "ACTIVE" if active else "inactive")
            was_active = active
        time.sleep(1)


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv[1:]))
    except KeyboardInterrupt:
        sys.exit(0)
