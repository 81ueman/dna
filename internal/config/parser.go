package config

import (
	"context"

	"github.com/81ueman/dna/internal/topology"
)

type Parser interface {
	LoadSnapshotDir(ctx context.Context, path string, topo topology.Topology) (Snapshot, error)
}

type NormalizedParser struct{}

func (NormalizedParser) LoadSnapshotDir(_ context.Context, path string, topo topology.Topology) (Snapshot, error) {
	return LoadSnapshotDir(path, topo)
}
