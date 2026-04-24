package topology

import (
	"strings"
	"testing"

	"github.com/81ueman/dna/internal/model"
)

func TestParseContainerlab(t *testing.T) {
	const input = `
name: demo
topology:
  nodes:
    r2:
      kind: ceos
    r1:
      kind: ceos
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
x-dna:
  edge_ports:
    - name: h2
      node: r2
      interface: eth2
      vrf: blue
    - name: h1
      node: r1
      interface: eth2
  node_config_names:
    r1: leaf1
    r2: leaf2
`

	topo, err := ParseContainerlab([]byte(input), LoadOptions{})
	if err != nil {
		t.Fatalf("parse containerlab: %v", err)
	}

	assertEqual(t, topo.Nodes, []model.Node{
		{ID: "r1"},
		{ID: "r2"},
	})
	assertEqual(t, topo.Links, []model.Link{
		{NodeA: "r1", InterfaceA: "eth1", NodeB: "r2", InterfaceB: "eth1"},
		{NodeA: "r2", InterfaceA: "eth1", NodeB: "r1", InterfaceB: "eth1"},
	})
	assertEqual(t, topo.EdgePorts, []model.EdgePort{
		{ID: "h1", Node: "r1", Interface: "eth2", VRF: model.DefaultVRF},
		{ID: "h2", Node: "r2", Interface: "eth2", VRF: "blue"},
	})
	assertEqual(t, topo.Interfaces, []model.Interface{
		{Node: "r1", ID: "eth1", VRF: model.DefaultVRF},
		{Node: "r1", ID: "eth2", VRF: model.DefaultVRF},
		{Node: "r2", ID: "eth1", VRF: model.DefaultVRF},
		{Node: "r2", ID: "eth2", VRF: "blue"},
	})

	if topo.NodeConfigNames["r1"] != "leaf1" {
		t.Fatalf("r1 config name = %q, want leaf1", topo.NodeConfigNames["r1"])
	}
	if topo.NodeConfigNames["r2"] != "leaf2" {
		t.Fatalf("r2 config name = %q, want leaf2", topo.NodeConfigNames["r2"])
	}
}

func TestParseContainerlabNormalizesInterfaces(t *testing.T) {
	const input = `
topology:
  nodes:
    r1: {}
    r2: {}
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
x-dna:
  edge_ports:
    - name: h1
      node: r1
      interface: eth2
`

	topo, err := ParseContainerlab([]byte(input), LoadOptions{
		NormalizeInterface: func(_ model.NodeID, iface model.InterfaceID) model.InterfaceID {
			return model.InterfaceID(strings.ReplaceAll(string(iface), "eth", "Ethernet"))
		},
	})
	if err != nil {
		t.Fatalf("parse containerlab: %v", err)
	}

	assertEqual(t, topo.Links, []model.Link{
		{NodeA: "r1", InterfaceA: "Ethernet1", NodeB: "r2", InterfaceB: "Ethernet1"},
		{NodeA: "r2", InterfaceA: "Ethernet1", NodeB: "r1", InterfaceB: "Ethernet1"},
	})
	assertEqual(t, topo.EdgePorts, []model.EdgePort{
		{ID: "h1", Node: "r1", Interface: "Ethernet2", VRF: model.DefaultVRF},
	})
}

func TestParseContainerlabValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "empty file",
			input:   "",
			wantErr: "topology.nodes",
		},
		{
			name: "missing nodes",
			input: `
topology:
  links: []
`,
			wantErr: "topology.nodes",
		},
		{
			name: "misspelled nodes",
			input: `
topology:
  nodez:
    r1: {}
`,
			wantErr: "topology.nodes",
		},
		{
			name: "malformed endpoint",
			input: `
topology:
  nodes:
    r1: {}
    r2: {}
  links:
    - endpoints: ["r1eth1", "r2:eth1"]
`,
			wantErr: "node:interface",
		},
		{
			name: "unknown link node",
			input: `
topology:
  nodes:
    r1: {}
  links:
    - endpoints: ["r1:eth1", "r2:eth1"]
`,
			wantErr: `unknown node "r2"`,
		},
		{
			name: "one ended link",
			input: `
topology:
  nodes:
    r1: {}
  links:
    - endpoints: ["r1:eth1"]
`,
			wantErr: "exactly 2 endpoints",
		},
		{
			name: "three ended link",
			input: `
topology:
  nodes:
    r1: {}
    r2: {}
    r3: {}
  links:
    - endpoints: ["r1:eth1", "r2:eth1", "r3:eth1"]
`,
			wantErr: "exactly 2 endpoints",
		},
		{
			name: "unknown edge node",
			input: `
topology:
  nodes:
    r1: {}
x-dna:
  edge_ports:
    - name: h1
      node: r2
      interface: eth1
`,
			wantErr: `unknown node "r2"`,
		},
		{
			name: "duplicate edge port",
			input: `
topology:
  nodes:
    r1: {}
x-dna:
  edge_ports:
    - name: h1
      node: r1
      interface: eth1
    - name: h1
      node: r1
      interface: eth2
`,
			wantErr: `duplicate edge port "h1"`,
		},
		{
			name: "missing edge field",
			input: `
topology:
  nodes:
    r1: {}
x-dna:
  edge_ports:
    - name: h1
      node: r1
`,
			wantErr: "missing interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseContainerlab([]byte(tt.input), LoadOptions{})
			if err == nil {
				t.Fatalf("ParseContainerlab succeeded, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
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
