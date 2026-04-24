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
