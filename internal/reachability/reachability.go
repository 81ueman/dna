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
	interfaceRules     map[nodeVRF][]model.ForwardingRule
}

func newIndex(topo topology.Topology, rules []model.ForwardingRule) (*graphIndex, error) {
	index := &graphIndex{
		nodes:              map[model.NodeID]bool{},
		interfaces:         map[interfaceKey]bool{},
		linksByInterface:   map[interfaceKey][]interfaceKey{},
		edgePortsByIngress: map[edgeKey][]model.EdgePort{},
		interfaceRules:     map[nodeVRF][]model.ForwardingRule{},
	}

	for _, node := range topo.Nodes {
		index.nodes[node.ID] = true
	}
	for _, iface := range topo.Interfaces {
		index.interfaces[interfaceKey{node: iface.Node, iface: iface.ID}] = true
	}
	for _, link := range topo.Links {
		from := interfaceKey{node: link.NodeA, iface: link.InterfaceA}
		to := interfaceKey{node: link.NodeB, iface: link.InterfaceB}
		if err := index.validateInterface(from, "link"); err != nil {
			return nil, err
		}
		if err := index.validateInterface(to, "link"); err != nil {
			return nil, err
		}
		index.linksByInterface[from] = append(index.linksByInterface[from], to)
	}
	for _, edge := range topo.EdgePorts {
		if err := index.validateInterface(interfaceKey{node: edge.Node, iface: edge.Interface}, "edge port"); err != nil {
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

	ecSeen := map[equivalenceClass]bool{}
	ruleSeen := map[model.ForwardingRule]bool{}
	for _, rule := range rules {
		if !index.nodes[rule.Node] {
			return nil, fmt.Errorf("forwarding rule references unknown node %q", rule.Node)
		}
		normalized := rule
		normalized.Prefix = rule.Prefix.Masked()
		if !ruleSeen[normalized] {
			ruleSeen[normalized] = true
			index.rules = append(index.rules, normalized)
		}
		ec := equivalenceClass{vrf: normalized.VRF, prefix: normalized.Prefix}
		if !ecSeen[ec] {
			ecSeen[ec] = true
			index.equivalenceClasses = append(index.equivalenceClasses, ec)
		}
		if normalized.Action == model.ForwardActionInterface {
			index.interfaceRules[nodeVRF{node: normalized.Node, vrf: normalized.VRF}] = append(
				index.interfaceRules[nodeVRF{node: normalized.Node, vrf: normalized.VRF}],
				normalized,
			)
		}
	}
	sortRules(index.rules)
	sortECs(index.equivalenceClasses)
	for key := range index.interfaceRules {
		sortRules(index.interfaceRules[key])
	}

	return index, nil
}

func (i *graphIndex) validateInterface(key interfaceKey, context string) error {
	if !i.nodes[key.node] {
		return fmt.Errorf("%s references unknown node %q", context, key.node)
	}
	if !i.interfaces[key] {
		return fmt.Errorf("%s references unknown interface %q on node %q", context, key.iface, key.node)
	}
	return nil
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
	defer delete(visited, key)

	for _, rule := range forwarding.Lookup(i.rules, node, ec.vrf, ec.prefix.Addr()) {
		switch rule.Action {
		case model.ForwardActionDrop:
			continue
		case model.ForwardActionInterface:
			i.exitInterface(source, rule.Node, rule.Interface, ec, visited, seen, reaches)
		case model.ForwardActionNextHop:
			for _, ifaceRule := range i.resolveNextHop(rule) {
				i.exitInterface(source, ifaceRule.Node, ifaceRule.Interface, ec, visited, seen, reaches)
			}
		}
	}
}

func (i *graphIndex) resolveNextHop(rule model.ForwardingRule) []model.ForwardingRule {
	var matches []model.ForwardingRule
	for _, ifaceRule := range i.interfaceRules[nodeVRF{node: rule.Node, vrf: rule.VRF}] {
		if ifaceRule.Prefix.Contains(rule.NextHop) {
			matches = append(matches, ifaceRule)
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

	for _, neighbor := range i.linksByInterface[interfaceKey{node: node, iface: iface}] {
		i.traverse(source, neighbor.node, ec, visited, seen, reaches)
	}
}

type interfaceKey struct {
	node  model.NodeID
	iface model.InterfaceID
}

type edgeKey struct {
	node  model.NodeID
	iface model.InterfaceID
	vrf   model.VRF
}

type nodeVRF struct {
	node model.NodeID
	vrf  model.VRF
}

type visitKey struct {
	node   model.NodeID
	vrf    model.VRF
	prefix netip.Prefix
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
		return a.iface < b.iface
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
