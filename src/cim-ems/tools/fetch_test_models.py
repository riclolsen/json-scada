"""Download public CGMES 2.4.15 conformity models (from powsybl-core test resources) as zip packages.

  python tools/fetch_test_models.py [destination] [--relicap]     (default: tests/data/conformity)

--relicap fetches ENTSO-E ReliCapGrid (CGMES 3.0, one package per region); other flags fetch the CGMES 2.4.15 cases.

The files are ENTSO-E conformity data distributed with PowSyBl. They are not committed
(the destination is gitignored); check their license before redistributing.
"""
import io
import json
import re
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


RELICAP_REPO = "entsoe/relicapgrid"
RELICAP_BRANCH = "cgmes-3.0_ncp-2.5_tc-2.1"
NOT_REGIONS = {"boundaryData", "commonData", "referenceData"}


def fetch_relicap(dest: Path) -> None:
    """ReliCapGrid (ENTSO-E, CGMES 3.0): one package per region with the boundary set.

    pypowsybl 1.16 does not resolve BaseVoltage/Region definitions kept in the separate common-data file, so
    its objects are merged into each region's EQ file here. This is dataset preparation, not importer logic.
    """
    tree = json.loads(get(f"https://api.github.com/repos/{RELICAP_REPO}/git/trees/{RELICAP_BRANCH}?recursive=1"))["tree"]
    raw = f"https://raw.githubusercontent.com/{RELICAP_REPO}/{RELICAP_BRANCH}/"
    grid = [t["path"] for t in tree if t["type"] == "blob" and re.match(r"Instance/[^/]+/Grid/cimxml/.*\.xml$", t["path"])]
    files = {p: get(raw + p).decode("utf-8") for p in grid}
    common = next(t for p, t in files.items() if "/commonData/" in p)
    body = common[common.index("</md:FullModel>") + len("</md:FullModel>"): common.rindex("</rdf:RDF>")]
    boundary = {p.rsplit("/", 1)[1]: t for p, t in files.items() if "/boundaryData/" in p}
    regions = sorted({p.split("/")[1] for p in grid} - NOT_REGIONS)
    for region in regions:
        mine = {p.rsplit("/", 1)[1]: t for p, t in files.items() if p.split("/")[1] == region}
        if not any("_EQ_" in n for n in mine):
            print(f"skipping {region}: no EQ file")
            continue
        with zipfile.ZipFile(dest / f"relicap_{region}.zip", "w", zipfile.ZIP_DEFLATED) as z:
            for name, text in mine.items():
                if "_EQ_" in name:
                    text = text[: text.rindex("</rdf:RDF>")] + body + "</rdf:RDF>\n"
                z.writestr(name, text)
            for name, text in boundary.items():
                z.writestr(name, text)
        print(f"relicap_{region}.zip")


def main() -> None:
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    dest = Path(args[0]) if args else Path(__file__).resolve().parents[1] / "tests/data/conformity"
    dest.mkdir(parents=True, exist_ok=True)
    if "--relicap" in sys.argv:
        fetch_relicap(dest)
        return
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
