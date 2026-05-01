import pytest

from batfish_exporter.cli import (
    ExportError,
    export_snapshot,
    export_static_routes,
    parse_interface_ref,
    staged_snapshot,
)


class FakeQuestion:
    def __init__(self, answer):
        self._answer = answer

    def answer(self):
        return self._answer


class FakeQuestions:
    def __init__(self, answer):
        self._answer = answer

    def viModel(self):
        return FakeQuestion(self._answer)


class FakeBatfish:
    def __init__(self, answer):
        self.q = FakeQuestions(answer)


def test_parse_interface_ref():
    assert parse_interface_ref("r1[Ethernet1]") == ("r1", "Ethernet1")


def test_parse_interface_ref_rejects_unexpected_format():
    with pytest.raises(ExportError, match="unexpected Batfish interface reference"):
        parse_interface_ref("r1 Ethernet1")


def test_export_snapshot_rejects_missing_snapshot_path(tmp_path):
    with pytest.raises(ExportError, match="snapshot path does not exist"):
        export_snapshot(tmp_path / "missing", host="localhost", network="dna", snapshot_name="snapshot")


def test_export_static_routes_uses_configured_vi_model_routes():
    routes = export_static_routes(
        FakeBatfish(
            {
                "answerElements": [
                    {
                        "nodes": {
                            "r1": {
                                "vrfs": {
                                    "default": {
                                        "staticRoutes": [
                                            {
                                                "network": "10.0.2.0/24",
                                                "nextHopInterface": "dynamic",
                                                "nextHopIp": "10.0.12.2",
                                            },
                                            {
                                                "network": "203.0.113.0/24",
                                                "nextHopInterface": "null_interface",
                                                "nextHopIp": "AUTO/NONE(-1l)",
                                            },
                                        ]
                                    }
                                }
                            }
                        }
                    }
                ]
            }
        )
    )

    assert routes == [
        {
            "node": "r1",
            "vrf": "default",
            "prefix": "10.0.2.0/24",
            "next_hop": "10.0.12.2",
        },
        {
            "node": "r1",
            "vrf": "default",
            "prefix": "203.0.113.0/24",
            "drop": True,
        },
    ]


def test_export_static_routes_preserves_unsupported_interface_next_hop():
    routes = export_static_routes(
        FakeBatfish(
            {
                "answerElements": [
                    {
                        "nodes": {
                            "r1": {
                                "vrfs": {
                                    "default": {
                                        "staticRoutes": [
                                            {
                                                "network": "10.0.2.0/24",
                                                "nextHopInterface": "Ethernet1",
                                                "nextHopIp": "AUTO/NONE(-1l)",
                                            }
                                        ]
                                    }
                                }
                            }
                        }
                    }
                ]
            }
        )
    )

    assert routes == [
        {
            "node": "r1",
            "vrf": "default",
            "prefix": "10.0.2.0/24",
            "next_hop_interface": "Ethernet1",
        }
    ]


def test_export_static_routes_reports_unexpected_vi_model_shape():
    with pytest.raises(ExportError, match="viModel answer"):
        export_static_routes(FakeBatfish({"answerElements": []}))


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
