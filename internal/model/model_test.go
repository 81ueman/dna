package model

import (
	"net/netip"
	"testing"
)

func mustPrefix(t *testing.T, raw string) netip.Prefix {
	t.Helper()

	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", raw, err)
	}

	return prefix.Masked()
}

func mustAddr(t *testing.T, raw string) netip.Addr {
	t.Helper()

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		t.Fatalf("parse addr %q: %v", raw, err)
	}

	return addr
}

func TestIdentifiersAreMapKeys(t *testing.T) {
	nodes := map[NodeID]Node{
		"r1": {ID: "r1"},
	}
	interfaces := map[InterfaceID]Interface{
		"Ethernet1": {Node: "r1", ID: "Ethernet1", VRF: DefaultVRF},
	}
	edgePorts := map[EdgePortID]EdgePort{
		"h1": {ID: "h1", Node: "r1", Interface: "Ethernet1", VRF: DefaultVRF},
	}

	if nodes["r1"].ID != "r1" {
		t.Fatalf("unexpected node lookup: %#v", nodes["r1"])
	}
	if interfaces["Ethernet1"].VRF != DefaultVRF {
		t.Fatalf("unexpected interface lookup: %#v", interfaces["Ethernet1"])
	}
	if edgePorts["h1"].Node != "r1" {
		t.Fatalf("unexpected edge port lookup: %#v", edgePorts["h1"])
	}
}

func TestDefaultVRF(t *testing.T) {
	if DefaultVRF != "default" {
		t.Fatalf("DefaultVRF = %q, want default", DefaultVRF)
	}
}

func TestPrefixNormalization(t *testing.T) {
	prefix := mustPrefix(t, "10.0.0.42/24")
	want := netip.MustParsePrefix("10.0.0.0/24")

	if prefix != want {
		t.Fatalf("normalized prefix = %s, want %s", prefix, want)
	}
}

func TestComparableFacts(t *testing.T) {
	route := StaticRoute{
		Node:    "r1",
		VRF:     DefaultVRF,
		Prefix:  mustPrefix(t, "10.0.0.0/24"),
		NextHop: mustAddr(t, "192.0.2.1"),
	}

	routes := map[StaticRoute]bool{route: true}
	if !routes[route] {
		t.Fatalf("static route was not usable as a map key")
	}
}

func TestFactStrings(t *testing.T) {
	prefix := mustPrefix(t, "10.0.0.0/24")
	nextHop := mustAddr(t, "192.0.2.1")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "node",
			got:  Node{ID: "r1"}.String(),
			want: "Node{ID:r1}",
		},
		{
			name: "interface",
			got:  Interface{Node: "r1", ID: "Ethernet1", VRF: DefaultVRF}.String(),
			want: "Interface{Node:r1 ID:Ethernet1 VRF:default}",
		},
		{
			name: "link",
			got: Link{
				NodeA:      "r1",
				InterfaceA: "Ethernet1",
				NodeB:      "r2",
				InterfaceB: "Ethernet2",
			}.String(),
			want: "Link{NodeA:r1 InterfaceA:Ethernet1 NodeB:r2 InterfaceB:Ethernet2}",
		},
		{
			name: "edge port",
			got: EdgePort{
				ID:        "h1",
				Node:      "r1",
				Interface: "Ethernet1",
				VRF:       DefaultVRF,
			}.String(),
			want: "EdgePort{ID:h1 Node:r1 Interface:Ethernet1 VRF:default}",
		},
		{
			name: "interface address",
			got: InterfaceAddress{
				Node:      "r1",
				Interface: "Ethernet1",
				VRF:       DefaultVRF,
				Prefix:    prefix,
			}.String(),
			want: "InterfaceAddress{Node:r1 Interface:Ethernet1 VRF:default Prefix:10.0.0.0/24}",
		},
		{
			name: "interface state",
			got:  InterfaceState{Node: "r1", Interface: "Ethernet1", Up: true}.String(),
			want: "InterfaceState{Node:r1 Interface:Ethernet1 Up:true}",
		},
		{
			name: "static route",
			got: StaticRoute{
				Node:    "r1",
				VRF:     DefaultVRF,
				Prefix:  prefix,
				NextHop: nextHop,
			}.String(),
			want: "StaticRoute{Node:r1 VRF:default Prefix:10.0.0.0/24 NextHop:192.0.2.1}",
		},
		{
			name: "connected route",
			got: ConnectedRoute{
				Node:      "r1",
				VRF:       DefaultVRF,
				Prefix:    prefix,
				Interface: "Ethernet1",
			}.String(),
			want: "ConnectedRoute{Node:r1 VRF:default Prefix:10.0.0.0/24 Interface:Ethernet1}",
		},
		{
			name: "forwarding rule",
			got: ForwardingRule{
				Node:      "r1",
				VRF:       DefaultVRF,
				Prefix:    prefix,
				Action:    ForwardActionNextHop,
				NextHop:   nextHop,
				Interface: "Ethernet1",
			}.String(),
			want: "ForwardingRule{Node:r1 VRF:default Prefix:10.0.0.0/24 Action:next-hop NextHop:192.0.2.1 Interface:Ethernet1}",
		},
		{
			name: "reach",
			got: Reach{
				Source: "h1",
				Dest:   "h2",
				VRF:    DefaultVRF,
				Prefix: prefix,
			}.String(),
			want: "Reach{Source:h1 Dest:h2 VRF:default Prefix:10.0.0.0/24}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("String() = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
