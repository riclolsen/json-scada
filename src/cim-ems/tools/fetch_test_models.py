"""Download public CGMES 2.4.15 conformity models (from powsybl-core test resources) as zip packages.

  python tools/fetch_test_models.py [destination]     (default: tests/data/conformity)

The files are ENTSO-E conformity data distributed with PowSyBl. They are not committed
(the destination is gitignored); check their license before redistributing.
"""
import io
import json
import sys
import urllib.request
import zipfile
from pathlib import Path

BASE = "https://api.github.com/repos/powsybl/powsybl-core/contents/cgmes/cgmes-conformity/src/main/resources/conformity/cas-1.1.3-data-4.0.3"
MODELS = {
    "smallgrid_nodebreaker": [
        "SmallGrid/NodeBreaker/CGMES_v2.4.15_SmallGridTestConfiguration_BaseCase_Complete_v3.0.0",
        "SmallGrid/NodeBreaker/CGMES_v2.4.15_SmallGridTestConfiguration_Boundary_v3.0.0",
    ],
    "microgrid_basecase": [
        "MicroGrid/BaseCase/CGMES_v2.4.15_MicroGridTestConfiguration_BC_Assembled_v2",
        "MicroGrid/BaseCase/CGMES_v2.4.15_MicroGridTestConfiguration_BD_v2",
    ],
}


def get(url: str) -> bytes:
    with urllib.request.urlopen(urllib.request.Request(url, headers={"User-Agent": "json-scada-cim-ems"}), timeout=60) as r:
        return r.read()


def main() -> None:
    dest = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parents[1] / "tests/data/conformity"
    dest.mkdir(parents=True, exist_ok=True)
    for name, folders in MODELS.items():
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as z:
            for folder in folders:
                for item in json.loads(get(f"{BASE}/{folder}")):
                    if item["type"] == "file":
                        z.writestr(item["name"], get(item["download_url"]))
        (dest / f"{name}.zip").write_bytes(buf.getvalue())
        print(f"{name}.zip {len(buf.getvalue())} bytes")


if __name__ == "__main__":
    main()
