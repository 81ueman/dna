# dna

`dna` is a prototype implementation of Differential Network Analysis, based on
the NSDI 2022 paper by Zhang et al.

The project goal is to report forwarding-behavior changes caused by network
control-plane changes. The first milestone is only the Go CLI scaffold. Routing
models, topology loading, Batfish parsing, reachability checking, and
incremental analysis are planned for later milestones.

## Design Direction

- Use Batfish only as a vendor configuration parser/normalizer.
- Use Containerlab topology files as the first topology input.
- Keep topology and parser adapters behind internal models.
- Include VRF in the core data model from the beginning.
- Start with static and connected routes, then add OSPF and BGP.
- Use prefix-based equivalence classes for the MVP.
- Implement the minimal DNA-specific Datalog/DDlog-like engine in Go.

## Current CLI

```sh
go run ./cmd/dna --help
go run ./cmd/dna diff --help
```

The planned first operator workflow is:

```sh
dna diff \
  --topology topology.clab.yaml \
  --old-configs configs/old \
  --new-configs configs/new
```

At this stage, `dna diff` is intentionally not implemented.

## Development

Run tests:

```sh
go test ./...
```

Run lint, if `golangci-lint` is installed:

```sh
golangci-lint run ./...
```

Run formatting:

```sh
go fmt ./...
```

The same commands are available as Make targets:

```sh
make test
make lint
make fmt
make run
```

## Batfish Parser Adapter

Batfish is used only as a vendor configuration parser/normalizer. The Go
forwarding, diff, and reachability code consume exported DNA facts and do not
call Batfish during recomputation.

The Python exporter is managed with `uv` under `tools/batfish-exporter`:

```sh
uv run --project tools/batfish-exporter batfish-export \
  --snapshot path/to/vendor/configs \
  --output /tmp/dna-batfish.json
```

If `path/to/vendor/configs` is not already a Batfish snapshot root containing a
`configs/` directory, the exporter stages it into that layout before uploading
it to Batfish.

Batfish itself must be running separately:

```sh
docker run --name batfish \
  -v batfish-data:/data \
  -p 8888:8888 \
  -p 9996:9996 \
  batfish/allinone
```

Run the exporter tests with:

```sh
uv run --project tools/batfish-exporter pytest
```

Run the Batfish integration test, with the Docker service running, using:

```sh
DNA_BATFISH_INTEGRATION=1 go test ./internal/config
```
