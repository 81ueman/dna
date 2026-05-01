package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/81ueman/dna/internal/topology"
)

func TestBatfishParserIntegration(t *testing.T) {
	if os.Getenv("DNA_BATFISH_INTEGRATION") != "1" {
		t.Skip("set DNA_BATFISH_INTEGRATION=1 with Batfish running to enable")
	}

	topo, err := topology.LoadContainerlab(
		filepath.Join("testdata", "batfish", "topology.clab.yaml"),
		topology.LoadOptions{},
	)
	if err != nil {
		t.Fatalf("load topology: %v", err)
	}

	want, err := LoadSnapshotDir(filepath.Join("testdata", "batfish", "normalized"), topo)
	if err != nil {
		t.Fatalf("load normalized snapshot: %v", err)
	}
	got, err := (BatfishParser{}).LoadSnapshotDir(
		context.Background(),
		filepath.Join("testdata", "batfish", "vendor"),
		topo,
	)
	if err != nil {
		t.Fatalf("load batfish snapshot: %v", err)
	}

	assertEqual(t, got.InterfaceAddresses, want.InterfaceAddresses)
	assertEqual(t, got.InterfaceStates, want.InterfaceStates)
	assertEqual(t, got.ConnectedRoutes, want.ConnectedRoutes)
	assertEqual(t, got.StaticRoutes, want.StaticRoutes)
}
