"""Model revision administration against the JSON-SCADA MongoDB.

  python tools/model_admin.py import <package.zip> <modelId> [--user NAME]
  python tools/model_admin.py list <modelId>
  python tools/model_admin.py activate <modelId> <revision>
  python tools/model_admin.py rollback <modelId>
"""
import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from cim_ems import mongo  # noqa: E402
from cim_ems.config import DEFAULT_CONFIG, load_main_config  # noqa: E402
from cim_ems.models import store  # noqa: E402
from cim_ems.models.importer import import_cgmes, validate  # noqa: E402


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--config", default=DEFAULT_CONFIG)
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("import")
    p.add_argument("package")
    p.add_argument("model_id")
    p.add_argument("--user", default="cli")
    sub.add_parser("list").add_argument("model_id")
    p = sub.add_parser("activate")
    p.add_argument("model_id")
    p.add_argument("revision", type=int)
    sub.add_parser("rollback").add_argument("model_id")
    a = ap.parse_args()

    db = mongo.connect(load_main_config(a.config))
    mongo.ensure_indexes(db)
    try:
        if a.cmd == "import":
            doc = store.register_import(db, a.package, a.model_id, validate(import_cgmes(a.package)), a.user)
            print(f"{a.model_id} revision {doc['revision']}: {doc['status']}")
            print(json.dumps(doc["validation"], indent=2, default=str))
            return 0 if doc["status"] != "rejected" else 1
        if a.cmd == "list":
            for d in db[mongo.MODELS].find({"modelId": a.model_id}).sort("revision"):
                print(d["revision"], d["status"], d["sourceSha256"][:12], d["importedAt"])
        elif a.cmd == "activate":
            print("active:", store.activate(db, a.model_id, a.revision)["revision"])
        elif a.cmd == "rollback":
            print("active:", store.rollback(db, a.model_id)["revision"])
    except store.ModelError as e:
        print("error:", e)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
