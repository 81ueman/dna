package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/81ueman/dna/internal/model"
)

func TestParseBatfishExport(t *testing.T) {
	const input = `{
  "nodes": ["leaf1", "leaf2"],
  "interfaces": [
    {
      "node": "leaf1",
      "interface": "Ethernet2",
      "vrf": "blue",
      "addresses": ["10.0.2.42/24"],
      "up": true
    },
    {
      "node": "leaf1",
      "interface": "Ethernet1",
      "vrf": "default",
      "addresses": ["10.0.12.1/30"],
      "up": false
    },
    {
      "node": "leaf2",
      "interface": "Ethernet1",
      "vrf": "default",
      "addresses": ["10.0.12.2/30"],
      "up": true
    }
  ],
  "static_routes": [
    {
      "node": "leaf1",
      "vrf": "blue",
      "prefix": "10.0.3.42/24",
      "next_hop": "10.0.12.2"
    },
    {
      "node": "leaf1",
      "vrf": "default",
      "prefix": "203.0.113.42/24",
      "drop": true
    }
  ]
}`

	snapshot, err := ParseBatfishExport([]byte(input), topologyWithConfigNames())
	if err != nil {
		t.Fatalf("parse batfish export: %v", err)
	}

	assertEqual(t, snapshot.InterfaceAddresses, []model.InterfaceAddress{
		{Node: "r1", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
		{Node: "r1", Interface: "Ethernet2", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24")},
		{Node: "r2", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
	})
	assertEqual(t, snapshot.InterfaceStates, []model.InterfaceState{
		{Node: "r1", Interface: "Ethernet1", Up: false},
		{Node: "r1", Interface: "Ethernet2", Up: true},
		{Node: "r2", Interface: "Ethernet1", Up: true},
	})
	assertEqual(t, snapshot.ConnectedRoutes, []model.ConnectedRoute{
		{Node: "r1", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24"), Interface: "Ethernet2"},
		{Node: "r2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30"), Interface: "Ethernet1"},
	})
	assertEqual(t, snapshot.StaticRoutes, []model.StaticRoute{
		{
			Node:    "r1",
			VRF:     "blue",
			Prefix:  mustPrefix(t, "10.0.3.0/24"),
			Action:  model.StaticRouteActionNextHop,
			NextHop: mustAddr(t, "10.0.12.2"),
		},
		{
			Node:   "r1",
			VRF:    model.DefaultVRF,
			Prefix: mustPrefix(t, "203.0.113.0/24"),
			Action: model.StaticRouteActionDrop,
		},
	})
}

func TestParseBatfishExportValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing topology node",
			input: `{
  "nodes": ["r1"],
  "interfaces": [
    {"node": "r1", "interface": "Ethernet1", "vrf": "default", "addresses": ["10.0.12.1/30"], "up": true}
  ],
  "static_routes": []
}`,
			wantErr: `missing config for topology node "r2"`,
		},
		{
			name: "unknown interface",
			input: `{
  "nodes": ["r1", "r2"],
  "interfaces": [
    {"node": "r1", "interface": "Ethernet99", "vrf": "default", "addresses": [], "up": true}
  ],
  "static_routes": []
}`,
			wantErr: `interface "Ethernet99"`,
		},
		{
			name: "invalid address",
			input: `{
  "nodes": ["r1", "r2"],
  "interfaces": [
    {"node": "r1", "interface": "Ethernet1", "vrf": "default", "addresses": ["not-a-prefix"], "up": true}
  ],
  "static_routes": []
}`,
			wantErr: "not-a-prefix",
		},
		{
			name: "unsupported interface next hop",
			input: `{
  "nodes": ["r1", "r2"],
  "interfaces": [],
  "static_routes": [
    {
      "node": "r1",
      "vrf": "default",
      "prefix": "10.0.2.0/24",
      "next_hop_interface": "Ethernet1"
    }
  ]
}`,
			wantErr: "unsupported interface next-hop",
		},
		{
			name: "next hop and drop",
			input: `{
  "nodes": ["r1", "r2"],
  "interfaces": [],
  "static_routes": [
    {
      "node": "r1",
      "vrf": "default",
      "prefix": "10.0.2.0/24",
      "next_hop": "10.0.12.2",
      "drop": true
    }
  ]
}`,
			wantErr: "exactly one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBatfishExport([]byte(tt.input), testTopology())
			if err == nil {
				t.Fatalf("ParseBatfishExport succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestBatfishParserReportsExporterFailure(t *testing.T) {
	dir := t.TempDir()
	fakeUV := filepath.Join(dir, "uv")
	if err := os.WriteFile(fakeUV, []byte("#!/bin/sh\necho 'batfish-export: pybatfish is not installed' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake uv: %v", err)
	}

	_, err := (BatfishParser{
		UV:         fakeUV,
		ProjectDir: dir,
	}).LoadSnapshotDir(context.Background(), dir, testTopology())
	if err == nil {
		t.Fatalf("LoadSnapshotDir succeeded, want exporter failure")
	}
	for _, want := range []string{"run batfish exporter", "pybatfish is not installed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want substring %q", err, want)
		}
	}
}
