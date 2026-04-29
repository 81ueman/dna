package forwarding

import (
	"net/netip"
	"testing"

	"github.com/81ueman/dna/internal/model"
)

func TestConnectedRoutesSuppressDisabledInterfaces(t *testing.T) {
	routes := ConnectedRoutes(
		[]model.InterfaceAddress{
			{Node: "r1", Interface: "Ethernet1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.1.1/24")},
			{Node: "r1", Interface: "Ethernet2", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.1/24")},
		},
		[]model.InterfaceState{
			{Node: "r1", Interface: "Ethernet1", Up: false},
			{Node: "r1", Interface: "Ethernet2", Up: true},
		},
	)

	assertEqual(t, routes, []model.ConnectedRoute{
		{Node: "r1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.2.0/24"), Interface: "Ethernet2"},
	})
}

func TestRulesFromStaticAndConnectedRoutes(t *testing.T) {
	rules := Rules(
		[]model.StaticRoute{
			{
				Node:    "r1",
				VRF:     model.DefaultVRF,
				Prefix:  mustPrefix(t, "10.0.3.0/24"),
				Action:  model.StaticRouteActionNextHop,
				NextHop: mustAddr(t, "192.0.2.1"),
			},
			{
				Node:   "r1",
				VRF:    "blue",
				Prefix: mustPrefix(t, "203.0.113.0/24"),
				Action: model.StaticRouteActionDrop,
			},
		},
		[]model.ConnectedRoute{
			{Node: "r1", VRF: model.DefaultVRF, Prefix: mustPrefix(t, "10.0.1.0/24"), Interface: "Ethernet1"},
		},
	)

	assertEqual(t, rules, []model.ForwardingRule{
		{
			Node:   "r1",
			VRF:    "blue",
			Prefix: mustPrefix(t, "203.0.113.0/24"),
			Action: model.ForwardActionDrop,
		},
		{
			Node:      "r1",
			VRF:       model.DefaultVRF,
			Prefix:    mustPrefix(t, "10.0.1.0/24"),
			Action:    model.ForwardActionInterface,
			Interface: "Ethernet1",
		},
		{
			Node:    "r1",
			VRF:     model.DefaultVRF,
			Prefix:  mustPrefix(t, "10.0.3.0/24"),
			Action:  model.ForwardActionNextHop,
			NextHop: mustAddr(t, "192.0.2.1"),
		},
	})
}

func TestLookupUsesLongestPrefixMatch(t *testing.T) {
	rules := []model.ForwardingRule{
		{
			Node:    "r1",
			VRF:     model.DefaultVRF,
			Prefix:  mustPrefix(t, "10.0.0.0/16"),
			Action:  model.ForwardActionNextHop,
			NextHop: mustAddr(t, "192.0.2.1"),
		},
		{
			Node:      "r1",
			VRF:       model.DefaultVRF,
			Prefix:    mustPrefix(t, "10.0.1.0/24"),
			Action:    model.ForwardActionInterface,
			Interface: "Ethernet1",
		},
		{
			Node:    "r1",
			VRF:     "blue",
			Prefix:  mustPrefix(t, "10.0.1.0/24"),
			Action:  model.ForwardActionNextHop,
			NextHop: mustAddr(t, "192.0.2.2"),
		},
	}

	matches := Lookup(rules, "r1", model.DefaultVRF, mustAddr(t, "10.0.1.42"))

	assertEqual(t, matches, []model.ForwardingRule{
		{
			Node:      "r1",
			VRF:       model.DefaultVRF,
			Prefix:    mustPrefix(t, "10.0.1.0/24"),
			Action:    model.ForwardActionInterface,
			Interface: "Ethernet1",
		},
	})
}

func TestLookupReturnsECMPTies(t *testing.T) {
	rules := Rules(
		[]model.StaticRoute{
			{
				Node:    "r1",
				VRF:     model.DefaultVRF,
				Prefix:  mustPrefix(t, "10.0.0.0/24"),
				Action:  model.StaticRouteActionNextHop,
				NextHop: mustAddr(t, "192.0.2.2"),
			},
			{
				Node:    "r1",
				VRF:     model.DefaultVRF,
				Prefix:  mustPrefix(t, "10.0.0.0/24"),
				Action:  model.StaticRouteActionNextHop,
				NextHop: mustAddr(t, "192.0.2.1"),
			},
		},
		nil,
	)

	matches := Lookup(rules, "r1", model.DefaultVRF, mustAddr(t, "10.0.0.42"))

	assertEqual(t, matches, []model.ForwardingRule{
		{
			Node:    "r1",
			VRF:     model.DefaultVRF,
			Prefix:  mustPrefix(t, "10.0.0.0/24"),
			Action:  model.ForwardActionNextHop,
			NextHop: mustAddr(t, "192.0.2.1"),
		},
		{
			Node:    "r1",
			VRF:     model.DefaultVRF,
			Prefix:  mustPrefix(t, "10.0.0.0/24"),
			Action:  model.ForwardActionNextHop,
			NextHop: mustAddr(t, "192.0.2.2"),
		},
	})
}

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
