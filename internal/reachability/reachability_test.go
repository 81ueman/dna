package reachability

import (
	"net/netip"
	"testing"

	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/topology"
)

func TestComputeStaticReachability(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", model.DefaultVRF, "10.0.2.0/24", "host"),
			nextHopRule("r1", model.DefaultVRF, "10.0.2.0/24", "10.0.12.2"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
}

func TestComputeDropsUnreachablePrefixes(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", model.DefaultVRF, "10.0.2.0/24", "host"),
			{
				Node:   "r1",
				VRF:    model.DefaultVRF,
				Prefix: mustPrefix(t, "10.0.2.0/24"),
				Action: model.ForwardActionDrop,
			},
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}
	if len(reaches) != 0 {
		t.Fatalf("reaches = %#v, want none", reaches)
	}
}

func TestComputeSplitsOverlappingPrefixes(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", model.DefaultVRF, "10.0.0.0/22", "host"),
			nextHopRule("r1", model.DefaultVRF, "10.0.0.0/22", "10.0.12.2"),
			{
				Node:   "r1",
				VRF:    model.DefaultVRF,
				Prefix: mustPrefix(t, "10.0.2.0/24"),
				Action: model.ForwardActionDrop,
			},
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.0.0/23")},
		{Source: "h1", Dest: "h2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.3.0/24")},
	})
}

func TestComputeResolvesRecursiveStaticNextHop(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", model.DefaultVRF, "10.0.2.0/24", "host"),
			nextHopRule("r1", model.DefaultVRF, "192.0.2.0/24", "10.0.12.2"),
			nextHopRule("r1", model.DefaultVRF, "10.0.2.0/24", "192.0.2.2"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
}

func TestComputeRecursiveNextHopLoopTerminates(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			nextHopRule("r1", model.DefaultVRF, "10.0.2.0/24", "192.0.2.2"),
			nextHopRule("r1", model.DefaultVRF, "192.0.2.0/24", "198.51.100.2"),
			nextHopRule("r1", model.DefaultVRF, "198.51.100.0/24", "192.0.2.2"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}
	if len(reaches) != 0 {
		t.Fatalf("reaches = %#v, want none", reaches)
	}
}

func TestComputeLoopTerminates(t *testing.T) {
	topo := topology.Topology{
		Nodes: []model.Node{{ID: "r1"}, {ID: "r2"}},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "r1-r2", VRF: model.DefaultVRF},
			{Node: "r2", ID: "r2-r1", VRF: model.DefaultVRF},
			{Node: "r1", ID: "host1", VRF: model.DefaultVRF},
			{Node: "r2", ID: "host2", VRF: model.DefaultVRF},
		},
		Links: []model.Link{
			{NodeA: "r1", InterfaceA: "r1-r2", NodeB: "r2", InterfaceB: "r2-r1"},
			{NodeA: "r2", InterfaceA: "r2-r1", NodeB: "r1", InterfaceB: "r1-r2"},
		},
		EdgePorts: []model.EdgePort{
			{ID: "h1", Node: "r1", Interface: "host1", VRF: model.DefaultVRF},
			{ID: "h2", Node: "r2", Interface: "host2", VRF: model.DefaultVRF},
		},
	}

	reaches, err := Compute(
		topo,
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			nextHopRule("r1", model.DefaultVRF, "10.0.9.0/24", "10.0.12.2"),
			nextHopRule("r2", model.DefaultVRF, "10.0.9.0/24", "10.0.12.1"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}
	if len(reaches) != 0 {
		t.Fatalf("reaches = %#v, want none", reaches)
	}
}

func TestComputeECMPReachesDestinationIfAnyPathWorks(t *testing.T) {
	reaches, err := Compute(
		twoRouterTopology(model.DefaultVRF),
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", model.DefaultVRF, "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", model.DefaultVRF, "10.0.2.0/24", "host"),
			nextHopRule("r1", model.DefaultVRF, "10.0.2.0/24", "10.0.12.2"),
			nextHopRule("r1", model.DefaultVRF, "10.0.2.0/24", "192.0.2.1"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
}

func TestComputeHonorsVRFIsolation(t *testing.T) {
	topo := twoRouterTopology("blue")
	reaches, err := Compute(
		topo,
		[]model.ForwardingRule{
			ifaceRule("r1", "blue", "10.0.12.0/30", "r1-r2"),
			ifaceRule("r2", "blue", "10.0.12.0/30", "r2-r1"),
			ifaceRule("r2", "blue", "10.0.2.0/24", "host"),
			nextHopRule("r1", "blue", "10.0.2.0/24", "10.0.12.2"),
			ifaceRule("r1", model.DefaultVRF, "10.0.99.0/24", "host"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
}

func TestComputeDoesNotTraverseLinkAcrossDifferentInterfaceVRFs(t *testing.T) {
	topo := topology.Topology{
		Nodes: []model.Node{{ID: "r1"}, {ID: "r2"}},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "r1-r2", VRF: "blue"},
			{Node: "r1", ID: "host", VRF: "blue"},
			{Node: "r2", ID: "r2-r1", VRF: model.DefaultVRF},
			{Node: "r2", ID: "host", VRF: "blue"},
		},
		Links: []model.Link{
			{NodeA: "r1", InterfaceA: "r1-r2", NodeB: "r2", InterfaceB: "r2-r1"},
		},
		EdgePorts: []model.EdgePort{
			{ID: "h1", Node: "r1", Interface: "host", VRF: "blue"},
			{ID: "h2", Node: "r2", Interface: "host", VRF: "blue"},
		},
	}

	reaches, err := Compute(
		topo,
		[]model.ForwardingRule{
			ifaceRule("r1", "blue", "10.0.2.0/24", "r1-r2"),
			ifaceRule("r2", "blue", "10.0.2.0/24", "host"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}
	if len(reaches) != 0 {
		t.Fatalf("reaches = %#v, want none", reaches)
	}
}

func TestComputeTraversesLinkOnlyWithinMatchingInterfaceVRF(t *testing.T) {
	topo := topology.Topology{
		Nodes: []model.Node{{ID: "r1"}, {ID: "r2"}},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "r1-r2", VRF: model.DefaultVRF},
			{Node: "r1", ID: "r1-r2", VRF: "blue"},
			{Node: "r1", ID: "host", VRF: "blue"},
			{Node: "r2", ID: "r2-r1", VRF: model.DefaultVRF},
			{Node: "r2", ID: "r2-r1", VRF: "blue"},
			{Node: "r2", ID: "host", VRF: "blue"},
		},
		Links: []model.Link{
			{NodeA: "r1", InterfaceA: "r1-r2", NodeB: "r2", InterfaceB: "r2-r1"},
		},
		EdgePorts: []model.EdgePort{
			{ID: "h1", Node: "r1", Interface: "host", VRF: "blue"},
			{ID: "h2", Node: "r2", Interface: "host", VRF: "blue"},
		},
	}

	reaches, err := Compute(
		topo,
		[]model.ForwardingRule{
			ifaceRule("r1", "blue", "10.0.2.0/24", "r1-r2"),
			ifaceRule("r2", "blue", "10.0.2.0/24", "host"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "h1", Dest: "h2", VRF: "blue", Prefix: mustPrefix(t, "10.0.2.0/24")},
	})
}

func TestComputeSortsReachFacts(t *testing.T) {
	topo := topology.Topology{
		Nodes: []model.Node{{ID: "r1"}},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "a", VRF: model.DefaultVRF},
			{Node: "r1", ID: "b", VRF: model.DefaultVRF},
			{Node: "r1", ID: "c", VRF: model.DefaultVRF},
		},
		EdgePorts: []model.EdgePort{
			{ID: "z", Node: "r1", Interface: "a", VRF: model.DefaultVRF},
			{ID: "a", Node: "r1", Interface: "b", VRF: model.DefaultVRF},
			{ID: "m", Node: "r1", Interface: "c", VRF: model.DefaultVRF},
		},
	}

	reaches, err := Compute(
		topo,
		[]model.ForwardingRule{
			ifaceRule("r1", model.DefaultVRF, "10.0.2.0/24", "b"),
			ifaceRule("r1", model.DefaultVRF, "10.0.3.0/24", "c"),
		},
	)
	if err != nil {
		t.Fatalf("Compute returned error: %v", err)
	}

	assertEqual(t, reaches, []model.Reach{
		{Source: "a", Dest: "m", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.3.0/24")},
		{Source: "m", Dest: "a", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24")},
		{Source: "z", Dest: "a", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24")},
		{Source: "z", Dest: "m", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.3.0/24")},
	})
}

func TestComputeRejectsInvalidTopologyReferences(t *testing.T) {
	_, err := Compute(
		topology.Topology{
			Nodes:      []model.Node{{ID: "r1"}},
			Interfaces: []model.Interface{{Node: "r1", ID: "eth1", VRF: model.DefaultVRF}},
			Links: []model.Link{
				{NodeA: "r1", InterfaceA: "eth1", NodeB: "r2", InterfaceB: "eth1"},
			},
		},
		nil,
	)
	if err == nil {
		t.Fatalf("Compute succeeded, want invalid topology error")
	}
}

func twoRouterTopology(vrf model.VRF) topology.Topology {
	return topology.Topology{
		Nodes: []model.Node{{ID: "r1"}, {ID: "r2"}},
		Interfaces: []model.Interface{
			{Node: "r1", ID: "r1-r2", VRF: vrf},
			{Node: "r1", ID: "host", VRF: vrf},
			{Node: "r2", ID: "r2-r1", VRF: vrf},
			{Node: "r2", ID: "host", VRF: vrf},
		},
		Links: []model.Link{
			{NodeA: "r1", InterfaceA: "r1-r2", NodeB: "r2", InterfaceB: "r2-r1"},
			{NodeA: "r2", InterfaceA: "r2-r1", NodeB: "r1", InterfaceB: "r1-r2"},
		},
		EdgePorts: []model.EdgePort{
			{ID: "h1", Node: "r1", Interface: "host", VRF: vrf},
			{ID: "h2", Node: "r2", Interface: "host", VRF: vrf},
		},
	}
}

func ifaceRule(node model.NodeID, vrf model.VRF, prefix string, iface model.InterfaceID) model.ForwardingRule {
	return model.ForwardingRule{
		Node:      node,
		VRF:       vrf,
		Prefix:    netip.MustParsePrefix(prefix).Masked(),
		Action:    model.ForwardActionInterface,
		Interface: iface,
	}
}

func nextHopRule(node model.NodeID, vrf model.VRF, prefix, nextHop string) model.ForwardingRule {
	return model.ForwardingRule{
		Node:    node,
		VRF:     vrf,
		Prefix:  netip.MustParsePrefix(prefix).Masked(),
		Action:  model.ForwardActionNextHop,
		NextHop: netip.MustParseAddr(nextHop),
	}
}

func mustPrefix(t *testing.T, raw string) netip.Prefix {
	t.Helper()
	return netip.MustParsePrefix(raw).Masked()
}

func assertEqual[T comparable](t *testing.T, got, want []T) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d\ngot:  %#v\nwant: %#v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("item %d = %#v, want %#v\nall got:  %#v\nall want: %#v", i, got[i], want[i], got, want)
		}
	}
}
