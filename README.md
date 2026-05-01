# dna

`dna` is a prototype implementation of Differential Network Analysis, based on
the NSDI 2022 paper by Zhang et al.

The project goal is to report forwarding-behavior changes caused by network
control-plane changes. The current MVP supports Containerlab topology files,
normalized YAML configuration snapshots, static and connected routes, full
reachability computation, and reachability diff output.

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
  --new-configs configs/new \
  --parser-backend normalized
```

Run the included static-route example:

```sh
go run ./cmd/dna diff \
  --topology examples/static/topology.clab.yaml \
  --old-configs examples/static/configs/old \
  --new-configs examples/static/configs/new
```

Expected output:

```text
+Reach(h1,h2,default,10.0.2.0/24)
```

The `normalized` parser backend is implemented. The `batfish` backend flag is
reserved for the Batfish parser adapter milestone and currently returns a clear
not-implemented error.

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
