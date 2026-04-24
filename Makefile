.PHONY: test lint fmt run

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

run:
	go run ./cmd/dna --help
