# Agent Notes

This repository is a Go prototype of Differential Network Analysis (DNA), based
on Zhang et al., NSDI 2022. The project goal is to report forwarding-behavior
changes caused by network control-plane changes.

## Current State

- Language: Go.
- CLI framework: Cobra.
- Entrypoint: `cmd/dna`.
- CLI package: `internal/cli`.
- Current implemented scope: scaffold only.
- `dna diff` exists as a command shape but intentionally returns "not
  implemented yet" when executed.
- The paper PDF in the repository is reference material and should not be
  modified unless explicitly requested.

## Planned Architecture

- Use Batfish only as a vendor configuration parser/normalizer.
- Keep Batfish out of incremental recomputation and property checking.
- Use Containerlab topology files as the first topology adapter.
- Keep the internal topology model independent of Containerlab.
- Include VRF in core models from the beginning.
- Start protocol support with static and connected routes, then add OSPF, then
  BGP.
- Use prefix-based equivalence classes for the MVP.
- Build a minimal DNA-specific Datalog/DDlog-like parser and evaluator in Go
  rather than depending on DDlog.

## Useful Commands

```sh
go test ./...
go run ./cmd/dna --help
go run ./cmd/dna diff --help
golangci-lint run ./...
```

`go test` and `go run` may need access to the standard Go build cache outside
the repository sandbox.

## Development Conventions

- Prefer small, issue-scoped changes.
- Keep issue #1 scaffold-only; do not add routing or topology behavior there.
- Place executable entrypoints under `cmd/`.
- Place project internals under `internal/`.
- Use `net/netip` for IP and prefix modeling when domain types are added.
- Keep CLI output deterministic so it can be used in golden tests.
- Run `go fmt ./...` before final verification.

## GitHub Issues

The roadmap is tracked in GitHub issues. The first implementation sequence is:

1. Scaffold Go CLI and project layout.
2. Define core fact schema and VRF-aware domain model.
3. Implement Containerlab topology adapter.
4. Add normalized config snapshot loader.
5. Build minimal DNA-specific Datalog parser and relation engine.
6. Derive static and connected forwarding rules.
7. Build forwarding graph and full reachability checker.
8. Implement snapshot diff output for reachability facts.
9. Add incremental delta engine and change-point traversal.
10. Implement Batfish parser adapter.
