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
  -p 9997:9997 \
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
