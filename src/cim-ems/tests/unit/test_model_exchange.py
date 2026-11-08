import json
import pypowsybl.network as pn
import pytest

from cim_ems.config import load_main_config, parse_args
from cim_ems.export import export_cgmes, round_trip
from cim_ems.models.importer import import_cgmes, sha256_file, validate


@pytest.fixture(scope="module")
def package(tmp_path_factory):
    f = tmp_path_factory.mktemp("cgmes") / "four.zip"
    export_cgmes(pn.create_four_substations_node_breaker_network(), f)
    return f


def test_import_keeps_node_breaker_topology(package):
    net = import_cgmes(package)
    assert len(net.get_switches()) == 59


def test_validation_flags_hvdc_and_converges(package):
    rep = validate(import_cgmes(package))
    assert rep.ok
    assert rep.load_flow["status"] == "CONVERGED"
    assert any("HVDC" in u for u in rep.unsupported)
    assert any("VSC" in u for u in rep.unsupported)


def test_round_trip_detects_vsc_reactive_loss():
    # Phase 0 finding: VSC reactive setpoint is lost on CGMES 2.4.15 round trip.
    res = round_trip(pn.create_four_substations_node_breaker_network())
    assert res["max_dp_mw"] < 0.5
    assert not res["ok"]
    assert res["max_dq_mvar"] > 100


def test_round_trip_exact_on_clean_network():
    res = round_trip(pn.create_eurostag_tutorial_example1_network())
    assert res["ok"], res["problems"]
    assert res["max_dp_mw"] < 1e-6 and res["max_dq_mvar"] < 1e-6


def test_round_trip_flags_lines_between_different_nominal_voltages():
    # IEEE 14 in pypowsybl has lines joining voltage levels of different nominal
    # voltage, which CGMES cannot express: not a valid exchange fixture.
    res = round_trip(pn.create_ieee14())
    assert not res["ok"]


def test_sha256_is_stable(package):
    assert sha256_file(package) == sha256_file(package)


def test_config(tmp_path):
    p = tmp_path / "c.json"
    p.write_text(json.dumps({"nodeName": "n", "mongoConnectionString": "mongodb://x", "mongoDatabaseName": "d"}))
    c = load_main_config(str(p))
    assert (c.node_name, c.database) == ("n", "d")
    p.write_text("{}")
    with pytest.raises(ValueError):
        load_main_config(str(p))
    assert parse_args(["2", "3", "x.json"]) == (2, 3, "x.json")
    assert parse_args([]) == (1, 1, "../conf/json-scada.json")


def test_detect_cgmes_version(tmp_path):
    import zipfile

    from cim_ems.models.importer import detect_cgmes_version

    for ns, expected in (
        ("http://iec.ch/TC57/CIM100#", "3.0"),
        ("http://iec.ch/TC57/2013/CIM-schema-cim16#", "2.4.15"),
        ("http://example.com/other#", "unknown"),
    ):
        f = tmp_path / f"{expected}.zip"
        with zipfile.ZipFile(f, "w") as z:
            z.writestr("x_EQ.xml", f'<rdf:RDF xmlns:cim="{ns}"></rdf:RDF>')
        assert detect_cgmes_version(f) == expected
