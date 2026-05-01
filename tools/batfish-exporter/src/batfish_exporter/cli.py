from __future__ import annotations

import argparse
import json
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Export Batfish-normalized facts as deterministic JSON."
    )
    parser.add_argument("--snapshot", required=True, help="Batfish snapshot root or config directory")
    parser.add_argument("--output", required=True, help="Output JSON path")
    parser.add_argument("--host", default="localhost", help="Batfish service host")
    parser.add_argument("--network", default="dna", help="Batfish network name")
    parser.add_argument("--snapshot-name", default="snapshot", help="Batfish snapshot name")
    args = parser.parse_args()

    try:
        export = export_snapshot(
            snapshot_path=Path(args.snapshot),
            host=args.host,
            network=args.network,
            snapshot_name=args.snapshot_name,
        )
    except ExportError as exc:
        print(f"batfish-export: {exc}", file=sys.stderr)
        return 1

    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(export, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return 0


class ExportError(Exception):
    pass


def export_snapshot(
    snapshot_path: Path,
    host: str,
    network: str,
    snapshot_name: str,
) -> dict[str, Any]:
    if not snapshot_path.exists():
        raise ExportError(f"snapshot path does not exist: {snapshot_path}")

    try:
        from pybatfish.client.session import Session
    except ImportError as exc:
        raise ExportError("pybatfish is not installed; run `uv sync --project tools/batfish-exporter`") from exc

    with staged_snapshot(snapshot_path) as staged:
        try:
            bf = Session(host=host)
            bf.set_network(network)
            bf.init_snapshot(str(staged), name=snapshot_name, overwrite=True)
        except Exception as exc:  # pybatfish wraps service errors in several exception types.
            raise ExportError(f"could not initialize Batfish snapshot via host {host!r}: {exc}") from exc

        parse_rows = question_rows(bf.q.fileParseStatus().answer().frame())
        bad_parse_rows = [row for row in parse_rows if str(row.get("Status")) != "PASSED"]
        if bad_parse_rows:
            details = ", ".join(
                f"{row.get('File_Name')}={row.get('Status')}" for row in bad_parse_rows
            )
            raise ExportError(f"Batfish did not fully parse all files: {details}")

        nodes = sorted(
            {
                str(node)
                for row in parse_rows
                for node in list_value(row.get("Nodes"))
                if str(node)
            }
        )

        interfaces = export_interfaces(bf)
        static_routes = export_static_routes(bf)

    return {
        "nodes": nodes,
        "interfaces": interfaces,
        "static_routes": static_routes,
    }


def export_interfaces(bf: Any) -> list[dict[str, Any]]:
    frame = bf.q.interfaceProperties(
        properties="Active,All_Prefixes,VRF",
        excludeShutInterfaces=False,
    ).answer().frame()
    interfaces: list[dict[str, Any]] = []
    for row in question_rows(frame):
        node, iface = parse_interface_ref(row.get("Interface"))
        addresses = sorted(str(prefix) for prefix in list_value(row.get("All_Prefixes")) if str(prefix))
        interfaces.append(
            {
                "node": node,
                "interface": iface,
                "vrf": default_vrf(row.get("VRF")),
                "addresses": addresses,
                "up": bool_value(row.get("Active")),
            }
        )
    return sorted(interfaces, key=lambda item: (item["node"], item["interface"], item["vrf"]))


def export_static_routes(bf: Any) -> list[dict[str, Any]]:
    frame = bf.q.routes(protocols="static").answer().frame()
    routes: list[dict[str, Any]] = []
    for row in question_rows(frame):
        route = {
            "node": str(row.get("Node")),
            "vrf": default_vrf(row.get("VRF")),
            "prefix": str(row.get("Network")),
        }
        next_hop_ip = string_or_empty(row.get("Next_Hop_IP"))
        next_hop_interface = string_or_empty(row.get("Next_Hop_Interface"))
        next_hop = string_or_empty(row.get("Next_Hop")).lower()

        if next_hop in {"discard", "drop"} or next_hop_interface == "null_interface":
            route["drop"] = True
        elif next_hop_ip:
            route["next_hop"] = next_hop_ip
        elif next_hop_interface:
            route["next_hop_interface"] = next_hop_interface

        routes.append(route)
    return sorted(
        routes,
        key=lambda item: (
            item["node"],
            item["vrf"],
            item["prefix"],
            item.get("next_hop", ""),
            item.get("next_hop_interface", ""),
            item.get("drop", False),
        ),
    )


class staged_snapshot:
    def __init__(self, path: Path):
        self.path = path
        self._tempdir: tempfile.TemporaryDirectory[str] | None = None

    def __enter__(self) -> Path:
        if (self.path / "configs").is_dir():
            return self.path

        self._tempdir = tempfile.TemporaryDirectory(prefix="dna-batfish-snapshot-")
        root = Path(self._tempdir.name)
        configs = root / "configs"
        configs.mkdir()
        for source in sorted(self.path.rglob("*")):
            if not source.is_file():
                continue
            relative = source.relative_to(self.path)
            target = configs / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, target)
        return root

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        if self._tempdir is not None:
            self._tempdir.cleanup()


def question_rows(frame: Any) -> list[dict[str, Any]]:
    return frame.where(frame.notnull(), None).to_dict(orient="records")


def parse_interface_ref(value: Any) -> tuple[str, str]:
    raw = str(value)
    node, sep, rest = raw.partition("[")
    if sep != "[" or not rest.endswith("]") or not node:
        raise ExportError(f"unexpected Batfish interface reference: {raw!r}")
    return node, rest[:-1]


def list_value(value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return value
    if isinstance(value, tuple):
        return list(value)
    if isinstance(value, set):
        return sorted(value)
    return [value]


def default_vrf(value: Any) -> str:
    raw = string_or_empty(value)
    return raw if raw else "default"


def string_or_empty(value: Any) -> str:
    if value is None:
        return ""
    raw = str(value)
    if raw in {"None", "nan", "<NA>"}:
        return ""
    return raw


def bool_value(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    raw = str(value).lower()
    return raw in {"true", "1", "yes"}


if __name__ == "__main__":
    raise SystemExit(main())
