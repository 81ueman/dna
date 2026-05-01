package reachability

import (
	"fmt"
	"net/netip"
	"sort"

	"github.com/81ueman/dna/internal/forwarding"
	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/topology"
)

func Compute(topo topology.Topology, rules []model.ForwardingRule) ([]model.Reach, error) {
	index, err := newIndex(topo, rules)
	if err != nil {
		return nil, err
	}

	seen := map[model.Reach]bool{}
	var reaches []model.Reach
	for _, edge := range index.edgePorts {
		for _, ec := range index.equivalenceClasses {
			if edge.VRF != ec.vrf {
				continue
			}
			index.traverse(edge, edge.Node, ec, map[visitKey]bool{}, seen, &reaches)
		}
	}

	sortReaches(reaches)
	return reaches, nil
}

type equivalenceClass struct {
	vrf    model.VRF
	prefix netip.Prefix
}

type graphIndex struct {
	nodes              map[model.NodeID]bool
	interfaces         map[interfaceKey]bool
	linksByInterface   map[interfaceKey][]interfaceKey
	edgePortsByIngress map[edgeKey][]model.EdgePort
	edgePorts          []model.EdgePort
	equivalenceClasses []equivalenceClass
	rules              []model.ForwardingRule
}

func newIndex(topo topology.Topology, rules []model.ForwardingRule) (*graphIndex, error) {
	index := &graphIndex{
		nodes:              map[model.NodeID]bool{},
		interfaces:         map[interfaceKey]bool{},
		linksByInterface:   map[interfaceKey][]interfaceKey{},
		edgePortsByIngress: map[edgeKey][]model.EdgePort{},
	}

	for _, node := range topo.Nodes {
		index.nodes[node.ID] = true
	}
	for _, iface := range topo.Interfaces {
		index.interfaces[interfaceKey{node: iface.Node, iface: iface.ID, vrf: iface.VRF}] = true
	}
	for _, link := range topo.Links {
		fromNodeIface := nodeInterface{node: link.NodeA, iface: link.InterfaceA}
		toNodeIface := nodeInterface{node: link.NodeB, iface: link.InterfaceB}
		if err := index.validateLinkedInterface(fromNodeIface, "link"); err != nil {
			return nil, err
		}
		if err := index.validateLinkedInterface(toNodeIface, "link"); err != nil {
			return nil, err
		}
		for _, vrf := range index.commonVRFs(fromNodeIface, toNodeIface) {
			from := interfaceKey{node: link.NodeA, iface: link.InterfaceA, vrf: vrf}
			to := interfaceKey{node: link.NodeB, iface: link.InterfaceB, vrf: vrf}
			index.linksByInterface[from] = append(index.linksByInterface[from], to)
		}
	}
	for _, edge := range topo.EdgePorts {
		if err := index.validateInterface(interfaceKey{node: edge.Node, iface: edge.Interface, vrf: edge.VRF}, "edge port"); err != nil {
			return nil, err
		}
		index.edgePorts = append(index.edgePorts, edge)
		key := edgeKey{node: edge.Node, iface: edge.Interface, vrf: edge.VRF}
		index.edgePortsByIngress[key] = append(index.edgePortsByIngress[key], edge)
	}
	sortEdgePorts(index.edgePorts)
	for key := range index.edgePortsByIngress {
		sortEdgePorts(index.edgePortsByIngress[key])
	}
	for key := range index.linksByInterface {
		sortInterfaces(index.linksByInterface[key])
	}

	ruleSeen := map[model.ForwardingRule]bool{}
	prefixesByVRF := map[model.VRF][]netip.Prefix{}
	prefixSeen := map[equivalenceClass]bool{}
	for _, rule := range rules {
		if !index.nodes[rule.Node] {
			return nil, fmt.Errorf("forwarding rule references unknown node %q", rule.Node)
		}
		normalized := rule
		normalized.Prefix = rule.Prefix.Masked()
		if normalized.Action == model.ForwardActionInterface {
			if err := index.validateInterface(
				interfaceKey{node: normalized.Node, iface: normalized.Interface, vrf: normalized.VRF},
				"forwarding rule",
			); err != nil {
				return nil, err
			}
		}
		if !ruleSeen[normalized] {
			ruleSeen[normalized] = true
			index.rules = append(index.rules, normalized)
		}
		prefixKey := equivalenceClass{vrf: normalized.VRF, prefix: normalized.Prefix}
		if !prefixSeen[prefixKey] {
			prefixSeen[prefixKey] = true
			prefixesByVRF[normalized.VRF] = append(prefixesByVRF[normalized.VRF], normalized.Prefix)
		}
	}
	sortRules(index.rules)
	for vrf := range prefixesByVRF {
		sortPrefixes(prefixesByVRF[vrf])
	}
	index.equivalenceClasses = buildEquivalenceClasses(prefixesByVRF)
	sortECs(index.equivalenceClasses)

	return index, nil
}

func (i *graphIndex) validateInterface(key interfaceKey, context string) error {
	if !i.nodes[key.node] {
		return fmt.Errorf("%s references unknown node %q", context, key.node)
	}
	if !i.interfaces[key] {
		return fmt.Errorf("%s references unknown interface %q on node %q in VRF %q", context, key.iface, key.node, key.vrf)
	}
	return nil
}

func (i *graphIndex) validateLinkedInterface(key nodeInterface, context string) error {
	if !i.nodes[key.node] {
		return fmt.Errorf("%s references unknown node %q", context, key.node)
	}
	for iface := range i.interfaces {
		if iface.node == key.node && iface.iface == key.iface {
			return nil
		}
	}
	return fmt.Errorf("%s references unknown interface %q on node %q", context, key.iface, key.node)
}

func (i *graphIndex) commonVRFs(a, b nodeInterface) []model.VRF {
	aVRFs := map[model.VRF]bool{}
	for iface := range i.interfaces {
		if iface.node == a.node && iface.iface == a.iface {
			aVRFs[iface.vrf] = true
		}
	}

	var vrfs []model.VRF
	for iface := range i.interfaces {
		if iface.node == b.node && iface.iface == b.iface && aVRFs[iface.vrf] {
			vrfs = append(vrfs, iface.vrf)
		}
	}
	sort.Slice(vrfs, func(i, j int) bool {
		return vrfs[i] < vrfs[j]
	})
	return vrfs
}

func (i *graphIndex) traverse(
	source model.EdgePort,
	node model.NodeID,
	ec equivalenceClass,
	visited map[visitKey]bool,
	seen map[model.Reach]bool,
	reaches *[]model.Reach,
) {
	key := visitKey{node: node, vrf: ec.vrf, prefix: ec.prefix}
	if visited[key] {
		return
	}
	visited[key] = true

	for _, rule := range forwarding.Lookup(i.rules, node, ec.vrf, ec.prefix.Addr()) {
		switch rule.Action {
		case model.ForwardActionDrop:
			continue
		case model.ForwardActionInterface:
			i.exitInterface(source, rule.Node, rule.Interface, ec, visited, seen, reaches)
		case model.ForwardActionNextHop:
			for _, ifaceRule := range i.resolveNextHop(rule.Node, rule.VRF, rule.NextHop, map[nextHopVisit]bool{}) {
				i.exitInterface(source, ifaceRule.Node, ifaceRule.Interface, ec, visited, seen, reaches)
			}
		}
	}
}

func (i *graphIndex) resolveNextHop(
	node model.NodeID,
	vrf model.VRF,
	nextHop netip.Addr,
	visited map[nextHopVisit]bool,
) []model.ForwardingRule {
	key := nextHopVisit{node: node, vrf: vrf, nextHop: nextHop}
	if visited[key] {
		return nil
	}
	visited[key] = true
	defer delete(visited, key)

	seen := map[model.ForwardingRule]bool{}
	var matches []model.ForwardingRule
	for _, rule := range forwarding.Lookup(i.rules, node, vrf, nextHop) {
		switch rule.Action {
		case model.ForwardActionDrop:
			continue
		case model.ForwardActionInterface:
			if !seen[rule] {
				seen[rule] = true
				matches = append(matches, rule)
			}
		case model.ForwardActionNextHop:
			for _, ifaceRule := range i.resolveNextHop(rule.Node, rule.VRF, rule.NextHop, visited) {
				if !seen[ifaceRule] {
					seen[ifaceRule] = true
					matches = append(matches, ifaceRule)
				}
			}
		}
	}
	sortRules(matches)
	return matches
}

func (i *graphIndex) exitInterface(
	source model.EdgePort,
	node model.NodeID,
	iface model.InterfaceID,
	ec equivalenceClass,
	visited map[visitKey]bool,
	seen map[model.Reach]bool,
	reaches *[]model.Reach,
) {
	if !i.interfaces[interfaceKey{node: node, iface: iface, vrf: ec.vrf}] {
		return
	}

	for _, dest := range i.edgePortsByIngress[edgeKey{node: node, iface: iface, vrf: ec.vrf}] {
		if dest.ID == source.ID {
			continue
		}
		reach := model.Reach{
			Source: source.ID,
			Dest:   dest.ID,
			VRF:    ec.vrf,
			Prefix: ec.prefix,
		}
		if !seen[reach] {
			seen[reach] = true
			*reaches = append(*reaches, reach)
		}
	}

	for _, neighbor := range i.linksByInterface[interfaceKey{node: node, iface: iface, vrf: ec.vrf}] {
		i.traverse(source, neighbor.node, ec, visited, seen, reaches)
	}
}

type nodeInterface struct {
	node  model.NodeID
	iface model.InterfaceID
}

type interfaceKey struct {
	node  model.NodeID
	iface model.InterfaceID
	vrf   model.VRF
}

type edgeKey struct {
	node  model.NodeID
	iface model.InterfaceID
	vrf   model.VRF
}

type nextHopVisit struct {
	node    model.NodeID
	vrf     model.VRF
	nextHop netip.Addr
}

type visitKey struct {
	node   model.NodeID
	vrf    model.VRF
	prefix netip.Prefix
}

func buildEquivalenceClasses(prefixesByVRF map[model.VRF][]netip.Prefix) []equivalenceClass {
	seen := map[equivalenceClass]bool{}
	var ecs []equivalenceClass
	for vrf, prefixes := range prefixesByVRF {
		for _, prefix := range prefixes {
			fragments := []netip.Prefix{prefix}
			for _, other := range prefixes {
				if !strictlyContainsPrefix(prefix, other) {
					continue
				}
				var next []netip.Prefix
				for _, fragment := range fragments {
					next = append(next, subtractPrefix(fragment, other)...)
				}
				fragments = next
			}
			for _, fragment := range fragments {
				ec := equivalenceClass{vrf: vrf, prefix: fragment}
				if !seen[ec] {
					seen[ec] = true
					ecs = append(ecs, ec)
				}
			}
		}
	}
	return ecs
}

func subtractPrefix(parent, child netip.Prefix) []netip.Prefix {
	parent = parent.Masked()
	child = child.Masked()
	if !strictlyContainsPrefix(parent, child) {
		return []netip.Prefix{parent}
	}

	var fragments []netip.Prefix
	var carve func(netip.Prefix)
	carve = func(current netip.Prefix) {
		current = current.Masked()
		if current == child {
			return
		}
		if !containsPrefix(current, child) {
			fragments = append(fragments, current)
			return
		}

		left, right := splitPrefix(current)
		if containsPrefix(left, child) {
			fragments = append(fragments, right)
			carve(left)
			return
		}
		fragments = append(fragments, left)
		carve(right)
	}
	carve(parent)
	sortPrefixes(fragments)
	return fragments
}

func strictlyContainsPrefix(parent, child netip.Prefix) bool {
	return containsPrefix(parent, child) && parent.Bits() < child.Bits()
}

func containsPrefix(parent, child netip.Prefix) bool {
	parent = parent.Masked()
	child = child.Masked()
	return parent.Addr().Is4() == child.Addr().Is4() &&
		parent.Bits() <= child.Bits() &&
		parent.Contains(child.Addr())
}

func splitPrefix(prefix netip.Prefix) (netip.Prefix, netip.Prefix) {
	prefix = prefix.Masked()
	nextBits := prefix.Bits() + 1
	left := netip.PrefixFrom(prefix.Addr(), nextBits).Masked()
	right := netip.PrefixFrom(setAddrBit(prefix.Addr(), prefix.Bits()), nextBits).Masked()
	return left, right
}

func setAddrBit(addr netip.Addr, bit int) netip.Addr {
	if addr.Is4() {
		bytes := addr.As4()
		bytes[bit/8] |= 1 << (7 - bit%8)
		return netip.AddrFrom4(bytes)
	}

	bytes := addr.As16()
	bytes[bit/8] |= 1 << (7 - bit%8)
	return netip.AddrFrom16(bytes)
}

func sortReaches(reaches []model.Reach) {
	sort.Slice(reaches, func(i, j int) bool {
		a, b := reaches[i], reaches[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Dest != b.Dest {
			return a.Dest < b.Dest
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		return comparePrefix(a.Prefix, b.Prefix) < 0
	})
}

func sortEdgePorts(edges []model.EdgePort) {
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.Interface != b.Interface {
			return a.Interface < b.Interface
		}
		return a.VRF < b.VRF
	})
}

func sortInterfaces(interfaces []interfaceKey) {
	sort.Slice(interfaces, func(i, j int) bool {
		a, b := interfaces[i], interfaces[j]
		if a.node != b.node {
			return a.node < b.node
		}
		if a.iface != b.iface {
			return a.iface < b.iface
		}
		return a.vrf < b.vrf
	})
}

func sortECs(ecs []equivalenceClass) {
	sort.Slice(ecs, func(i, j int) bool {
		a, b := ecs[i], ecs[j]
		if a.vrf != b.vrf {
			return a.vrf < b.vrf
		}
		return comparePrefix(a.prefix, b.prefix) < 0
	})
}

func sortPrefixes(prefixes []netip.Prefix) {
	sort.Slice(prefixes, func(i, j int) bool {
		return comparePrefix(prefixes[i], prefixes[j]) < 0
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
