import pytest

from batfish_exporter.cli import ExportError, export_snapshot, parse_interface_ref, staged_snapshot


def test_parse_interface_ref():
    assert parse_interface_ref("r1[Ethernet1]") == ("r1", "Ethernet1")


def test_parse_interface_ref_rejects_unexpected_format():
    with pytest.raises(ExportError, match="unexpected Batfish interface reference"):
        parse_interface_ref("r1 Ethernet1")


def test_export_snapshot_rejects_missing_snapshot_path(tmp_path):
    with pytest.raises(ExportError, match="snapshot path does not exist"):
        export_snapshot(tmp_path / "missing", host="localhost", network="dna", snapshot_name="snapshot")


def test_staged_snapshot_wraps_config_directory(tmp_path):
    source = tmp_path / "vendor"
    source.mkdir()
    (source / "r1.cfg").write_text("hostname r1\n", encoding="utf-8")

    with staged_snapshot(source) as staged:
        staged_file = staged / "configs" / "r1.cfg"
        assert staged_file.read_text(encoding="utf-8") == "hostname r1\n"


def test_staged_snapshot_keeps_existing_snapshot_root(tmp_path):
    snapshot = tmp_path / "snapshot"
    configs = snapshot / "configs"
    configs.mkdir(parents=True)

    with staged_snapshot(snapshot) as staged:
        assert staged == snapshot
