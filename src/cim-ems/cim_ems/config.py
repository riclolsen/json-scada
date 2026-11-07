"""Command line arguments and conf/json-scada.json, as for the other drivers."""
import json
from dataclasses import dataclass
from pathlib import Path

DEFAULT_CONFIG = "../conf/json-scada.json"


@dataclass(frozen=True)
class MainConfig:
    node_name: str
    mongo_uri: str
    database: str


def load_main_config(path: str = DEFAULT_CONFIG) -> MainConfig:
    raw = json.loads(Path(path).read_text(encoding="utf-8"))
    for key in ("nodeName", "mongoConnectionString", "mongoDatabaseName"):
        if not raw.get(key):
            raise ValueError(f"{path}: missing '{key}'")
    return MainConfig(raw["nodeName"], raw["mongoConnectionString"], raw["mongoDatabaseName"])


def parse_args(argv: list[str]) -> tuple[int, int, str]:
    """`<instance> <logLevel> <configFile>`, all optional, like the other drivers."""
    instance = int(argv[0]) if len(argv) > 0 else 1
    log_level = int(argv[1]) if len(argv) > 1 else 1
    config = argv[2] if len(argv) > 2 else DEFAULT_CONFIG
    return instance, log_level, config
