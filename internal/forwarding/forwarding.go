package forwarding

import (
	"net/netip"
	"sort"

	"github.com/81ueman/dna/internal/model"
)

// ConnectedRoutes derives connected routes only for interfaces that are up.
// Interfaces without an explicit state are treated as up so tests and adapters
// can pass partial fact sets.
func ConnectedRoutes(
	addresses []model.InterfaceAddress,
	states []model.InterfaceState,
) []model.ConnectedRoute {
	up := map[interfaceKey]bool{}
	for _, state := range states {
		up[interfaceKey{node: state.Node, iface: state.Interface}] = state.Up
	}

	seen := map[model.ConnectedRoute]bool{}
	var routes []model.ConnectedRoute
	for _, address := range addresses {
		if stateUp, ok := up[interfaceKey{node: address.Node, iface: address.Interface}]; ok && !stateUp {
			continue
		}

		route := model.ConnectedRoute{
			Node:      address.Node,
			VRF:       address.VRF,
			Prefix:    address.Prefix.Masked(),
			Interface: address.Interface,
		}
		if seen[route] {
			continue
		}
		seen[route] = true
		routes = append(routes, route)
	}

	sortConnectedRoutes(routes)
	return routes
}

func Rules(
	staticRoutes []model.StaticRoute,
	connectedRoutes []model.ConnectedRoute,
) []model.ForwardingRule {
	seen := map[model.ForwardingRule]bool{}
	var rules []model.ForwardingRule

	for _, route := range connectedRoutes {
		rule := model.ForwardingRule{
			Node:      route.Node,
			VRF:       route.VRF,
			Prefix:    route.Prefix.Masked(),
			Action:    model.ForwardActionInterface,
			Interface: route.Interface,
		}
		if seen[rule] {
			continue
		}
		seen[rule] = true
		rules = append(rules, rule)
	}

	for _, route := range staticRoutes {
		rule := model.ForwardingRule{
			Node:   route.Node,
			VRF:    route.VRF,
			Prefix: route.Prefix.Masked(),
		}
		switch route.Action {
		case model.StaticRouteActionDrop:
			rule.Action = model.ForwardActionDrop
		default:
			rule.Action = model.ForwardActionNextHop
			rule.NextHop = route.NextHop
		}
		if seen[rule] {
			continue
		}
		seen[rule] = true
		rules = append(rules, rule)
	}

	sortRules(rules)
	return rules
}

func RulesFromFacts(
	addresses []model.InterfaceAddress,
	states []model.InterfaceState,
	staticRoutes []model.StaticRoute,
) []model.ForwardingRule {
	return Rules(staticRoutes, ConnectedRoutes(addresses, states))
}

func Lookup(
	rules []model.ForwardingRule,
	node model.NodeID,
	vrf model.VRF,
	destination netip.Addr,
) []model.ForwardingRule {
	bestBits := -1
	var matches []model.ForwardingRule
	for _, rule := range rules {
		if rule.Node != node || rule.VRF != vrf || !rule.Prefix.Contains(destination) {
			continue
		}
		bits := rule.Prefix.Bits()
		if bits < bestBits {
			continue
		}
		if bits > bestBits {
			bestBits = bits
			matches = matches[:0]
		}
		matches = append(matches, rule)
	}

	sortRules(matches)
	return matches
}

type interfaceKey struct {
	node  model.NodeID
	iface model.InterfaceID
}

func sortConnectedRoutes(routes []model.ConnectedRoute) {
	sort.Slice(routes, func(i, j int) bool {
		a, b := routes[i], routes[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		if comparePrefix(a.Prefix, b.Prefix) != 0 {
			return comparePrefix(a.Prefix, b.Prefix) < 0
		}
		return a.Interface < b.Interface
	})
}

func sortRules(rules []model.ForwardingRule) {
	sort.Slice(rules, func(i, j int) bool {
		a, b := rules[i], rules[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		if comparePrefix(a.Prefix, b.Prefix) != 0 {
			return comparePrefix(a.Prefix, b.Prefix) < 0
		}
		if a.Action != b.Action {
			return a.Action < b.Action
		}
		if a.NextHop.Compare(b.NextHop) != 0 {
			return a.NextHop.Compare(b.NextHop) < 0
		}
		return a.Interface < b.Interface
	})
}

func comparePrefix(a, b netip.Prefix) int {
	if cmp := a.Addr().Compare(b.Addr()); cmp != 0 {
		return cmp
	}
	if a.Bits() < b.Bits() {
		return -1
	}
	if a.Bits() > b.Bits() {
		return 1
	}
	return 0
}
