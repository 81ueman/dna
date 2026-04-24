package config

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/topology"
)

func TestParseNodeConfig(t *testing.T) {
	const input = `
node: r1
interfaces:
  Ethernet2:
    vrf: blue
    addresses:
      - 10.0.2.42/24
  Ethernet1:
    addresses:
      - 10.0.12.1/30
    up: false
static_routes:
  - vrf: blue
    prefix: 10.0.3.42/24
    next_hop: 10.0.12.2
  - prefix: 203.0.113.42/24
    drop: true
`

	snapshot, err := ParseNodeConfig([]byte(input), testTopology())
	if err != nil {
		t.Fatalf("parse node config: %v", err)
	}

	assertEqual(t, snapshot.InterfaceAddresses, []model.InterfaceAddress{
		{Node: "r1", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
		{Node: "r1", Interface: "Ethernet2", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
	assertEqual(t, snapshot.InterfaceStates, []model.InterfaceState{
		{Node: "r1", Interface: "Ethernet1", Up: false},
		{Node: "r1", Interface: "Ethernet2", Up: true},
	})
	assertEqual(t, snapshot.ConnectedRoutes, []model.ConnectedRoute{
		{Node: "r1", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24"), Interface: "Ethernet2"},
		{Node: "r1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30"), Interface: "Ethernet1"},
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

func TestLoadSnapshotDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "r2.yml", `
node: r2
interfaces:
  Ethernet1:
    addresses:
      - 10.0.12.2/30
static_routes:
  - prefix: 10.0.1.0/24
    next_hop: 10.0.12.1
`)
	writeFile(t, dir, "r1.yaml", `
node: r1
interfaces:
  Ethernet1:
    addresses:
      - 10.0.12.1/30
`)
	writeFile(t, dir, "ignored.txt", `
not yaml
`)

	snapshot, err := LoadSnapshotDir(dir, testTopology())
	if err != nil {
		t.Fatalf("load snapshot dir: %v", err)
	}

	assertEqual(t, snapshot.InterfaceAddresses, []model.InterfaceAddress{
		{Node: "r1", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
		{Node: "r2", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
	})
	assertEqual(t, snapshot.InterfaceStates, []model.InterfaceState{
		{Node: "r1", Interface: "Ethernet1", Up: true},
		{Node: "r2", Interface: "Ethernet1", Up: true},
	})
	assertEqual(t, snapshot.ConnectedRoutes, []model.ConnectedRoute{
		{Node: "r1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30"), Interface: "Ethernet1"},
		{Node: "r2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30"), Interface: "Ethernet1"},
	})
	assertEqual(t, snapshot.StaticRoutes, []model.StaticRoute{
		{
			Node:    "r2",
			VRF:     model.DefaultVRF,
			Prefix:  mustPrefix(t, "10.0.1.0/24"),
			Action:  model.StaticRouteActionNextHop,
			NextHop: mustAddr(t, "10.0.12.1"),
		},
	})
}

func TestLoadSnapshotDirAllowsEmptyNodeConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "r1.yaml", `
node: r1
interfaces:
  Ethernet1:
    addresses:
      - 10.0.12.1/30
`)
	writeFile(t, dir, "r2.yaml", `
node: r2
`)

	snapshot, err := LoadSnapshotDir(dir, testTopology())
	if err != nil {
		t.Fatalf("load snapshot dir: %v", err)
	}

	assertEqual(t, snapshot.InterfaceAddresses, []model.InterfaceAddress{
		{Node: "r1", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30")},
	})
	assertEqual(t, snapshot.InterfaceStates, []model.InterfaceState{
		{Node: "r1", Interface: "Ethernet1", Up: true},
	})
	assertEqual(t, snapshot.ConnectedRoutes, []model.ConnectedRoute{
		{Node: "r1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.12.0/30"), Interface: "Ethernet1"},
	})
	assertEqual(t, snapshot.StaticRoutes, nil)
}

func TestLoadSnapshotDirDuplicateNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", `
node: r1
interfaces:
  Ethernet1: {}
`)
	writeFile(t, dir, "b.yaml", `
node: r1
interfaces:
  Ethernet1: {}
`)

	_, err := LoadSnapshotDir(dir, testTopology())
	if err == nil {
		t.Fatalf("LoadSnapshotDir succeeded, want duplicate node error")
	}
	if !strings.Contains(err.Error(), `duplicate node "r1"`) {
		t.Fatalf("error = %q, want duplicate node", err)
	}
}

func TestLoadSnapshotDirMissingTopologyNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "r1.yaml", `
node: r1
interfaces:
  Ethernet1: {}
`)

	_, err := LoadSnapshotDir(dir, testTopology())
	if err == nil {
		t.Fatalf("LoadSnapshotDir succeeded, want missing node error")
	}
	if !strings.Contains(err.Error(), `missing config for topology node "r2"`) {
		t.Fatalf("error = %q, want missing topology node", err)
	}
}

func TestParseNodeConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "missing node",
			input: `
interfaces:
  Ethernet1: {}
`,
			wantErr: "node is required",
		},
		{
			name: "unknown node",
			input: `
node: r3
interfaces:
  Ethernet1: {}
`,
			wantErr: `node "r3" not found`,
		},
		{
			name: "unknown interface",
			input: `
node: r1
interfaces:
  Ethernet99: {}
`,
			wantErr: `interface "Ethernet99"`,
		},
		{
			name: "unknown interface vrf",
			input: `
node: r1
interfaces:
  Ethernet2:
    vrf: default
`,
			wantErr: `VRF "default"`,
		},
		{
			name: "unknown static route vrf",
			input: `
node: r2
static_routes:
  - vrf: blue
    prefix: 10.0.2.0/24
    next_hop: 10.0.12.1
`,
			wantErr: `VRF "blue"`,
		},
		{
			name: "invalid interface prefix",
			input: `
node: r1
interfaces:
  Ethernet1:
    addresses:
      - not-a-prefix
`,
			wantErr: "not-a-prefix",
		},
		{
			name: "invalid static prefix",
			input: `
node: r1
static_routes:
  - prefix: not-a-prefix
    next_hop: 10.0.12.2
`,
			wantErr: "not-a-prefix",
		},
		{
			name: "invalid next hop",
			input: `
node: r1
static_routes:
  - prefix: 10.0.2.0/24
    next_hop: not-an-ip
`,
			wantErr: "not-an-ip",
		},
		{
			name: "next hop and drop",
			input: `
node: r1
static_routes:
  - prefix: 10.0.2.0/24
    next_hop: 10.0.12.2
    drop: true
`,
			wantErr: "exactly one",
		},
		{
			name: "neither next hop nor drop",
			input: `
node: r1
static_routes:
  - prefix: 10.0.2.0/24
`,
			wantErr: "exactly one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseNodeConfig([]byte(tt.input), testTopology())
			if err == nil {
				t.Fatalf("ParseNodeConfig succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func testTopology() topology.Topology {
	return topology.Topology{
		Nodes: []model.Node{
			{ID: "r1"},
			{ID: "r2"},
		},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "Ethernet1", VRF: model.DefaultVRF},
			{Node: "r1", ID: "Ethernet2", VRF: "blue"},
			{Node: "r2", ID: "Ethernet1", VRF: model.DefaultVRF},
		},
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func mustPrefix(t *testing.T, raw string) netip.Prefix {
	t.Helper()

	prefix, err := parsePrefix(raw)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", raw, err)
	}
	return prefix
}

func mustAddr(t *testing.T, raw string) netip.Addr {
	t.Helper()

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		t.Fatalf("parse addr %q: %v", raw, err)
	}
	return addr
}

func assertEqual[T comparable](t *testing.T, got, want []T) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d\ngot:  %#v\nwant: %#v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("item %d = %#v, want %#v\ngot:  %#v\nwant: %#v", i, got[i], want[i], got, want)
		}
	}
}
