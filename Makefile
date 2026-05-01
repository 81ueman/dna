.PHONY: test lint fmt run batfish-exporter-test batfish-integration-test

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

run:
	go run ./cmd/dna --help

batfish-exporter-test:
	uv run --project tools/batfish-exporter pytest

batfish-integration-test:
	DNA_BATFISH_INTEGRATION=1 go test ./internal/config
