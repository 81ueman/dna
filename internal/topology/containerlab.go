package topology

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/81ueman/dna/internal/model"
	"gopkg.in/yaml.v3"
)

type InterfaceNormalizer func(node model.NodeID, iface model.InterfaceID) model.InterfaceID

type LoadOptions struct {
	NormalizeInterface InterfaceNormalizer
}

type Topology struct {
	Nodes           []model.Node
	Interfaces      []model.Interface
	Links           []model.Link
	EdgePorts       []model.EdgePort
	NodeConfigNames map[model.NodeID]string
}

func LoadContainerlab(path string, opts LoadOptions) (Topology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Topology{}, fmt.Errorf("read containerlab topology: %w", err)
	}

	return ParseContainerlab(data, opts)
}

func ParseContainerlab(data []byte, opts LoadOptions) (Topology, error) {
	var input containerlabFile
	if err := yaml.Unmarshal(data, &input); err != nil {
		return Topology{}, fmt.Errorf("parse containerlab topology: %w", err)
	}

	normalize := opts.NormalizeInterface
	if normalize == nil {
		normalize = func(_ model.NodeID, iface model.InterfaceID) model.InterfaceID {
			return iface
		}
	}

	if len(input.Topology.Nodes) == 0 {
		return Topology{}, fmt.Errorf("topology.nodes must declare at least one node")
	}

	builder := newBuilder(normalize)
	for nodeName := range input.Topology.Nodes {
		if nodeName == "" {
			return Topology{}, fmt.Errorf("node name must not be empty")
		}
		builder.addNode(model.NodeID(nodeName))
	}

	for i, link := range input.Topology.Links {
		if len(link.Endpoints) != 2 {
			return Topology{}, fmt.Errorf("link %d must have exactly 2 endpoints, got %d", i, len(link.Endpoints))
		}

		a, err := parseEndpoint(link.Endpoints[0], normalize)
		if err != nil {
			return Topology{}, fmt.Errorf("link %d endpoint 0: %w", i, err)
		}
		b, err := parseEndpoint(link.Endpoints[1], normalize)
		if err != nil {
			return Topology{}, fmt.Errorf("link %d endpoint 1: %w", i, err)
		}
		if !builder.hasNode(a.node) {
			return Topology{}, fmt.Errorf("link %d references unknown node %q", i, a.node)
		}
		if !builder.hasNode(b.node) {
			return Topology{}, fmt.Errorf("link %d references unknown node %q", i, b.node)
		}

		builder.addInterface(a.node, a.iface, model.DefaultVRF)
		builder.addInterface(b.node, b.iface, model.DefaultVRF)
		builder.addLink(a.node, a.iface, b.node, b.iface)
		builder.addLink(b.node, b.iface, a.node, a.iface)
	}

	for i, edgePort := range input.XDNA.EdgePorts {
		edge, err := edgePort.toModel(i, normalize)
		if err != nil {
			return Topology{}, err
		}
		if !builder.hasNode(edge.Node) {
			return Topology{}, fmt.Errorf("edge port %q references unknown node %q", edge.ID, edge.Node)
		}
		if builder.hasEdgePort(edge.ID) {
			return Topology{}, fmt.Errorf("duplicate edge port %q", edge.ID)
		}

		builder.addInterface(edge.Node, edge.Interface, edge.VRF)
		builder.addEdgePort(edge)
	}

	for nodeName, configName := range input.XDNA.NodeConfigNames {
		node := model.NodeID(nodeName)
		if !builder.hasNode(node) {
			return Topology{}, fmt.Errorf("node config mapping references unknown node %q", node)
		}
		builder.addNodeConfigName(node, configName)
	}

	return builder.build(), nil
}

type containerlabFile struct {
	Topology struct {
		Nodes map[string]yaml.Node `yaml:"nodes"`
		Links []struct {
			Endpoints []string `yaml:"endpoints"`
		} `yaml:"links"`
	} `yaml:"topology"`
	XDNA struct {
		EdgePorts       []edgePortYAML    `yaml:"edge_ports"`
		NodeConfigNames map[string]string `yaml:"node_config_names"`
	} `yaml:"x-dna"`
}

type edgePortYAML struct {
	Name      string `yaml:"name"`
	Node      string `yaml:"node"`
	Interface string `yaml:"interface"`
	VRF       string `yaml:"vrf"`
}

func (e edgePortYAML) toModel(index int, normalize InterfaceNormalizer) (model.EdgePort, error) {
	if e.Name == "" {
		return model.EdgePort{}, fmt.Errorf("edge port %d missing name", index)
	}
	if e.Node == "" {
		return model.EdgePort{}, fmt.Errorf("edge port %q missing node", e.Name)
	}
	if e.Interface == "" {
		return model.EdgePort{}, fmt.Errorf("edge port %q missing interface", e.Name)
	}

	node := model.NodeID(e.Node)
	vrf := model.VRF(e.VRF)
	if vrf == "" {
		vrf = model.DefaultVRF
	}

	return model.EdgePort{
		ID:        model.EdgePortID(e.Name),
		Node:      node,
		Interface: normalize(node, model.InterfaceID(e.Interface)),
		VRF:       vrf,
	}, nil
}

type endpoint struct {
	node  model.NodeID
	iface model.InterfaceID
}

func parseEndpoint(raw string, normalize InterfaceNormalizer) (endpoint, error) {
	nodeName, ifaceName, ok := strings.Cut(raw, ":")
	if !ok {
		return endpoint{}, fmt.Errorf("endpoint %q must be in node:interface form", raw)
	}
	if nodeName == "" {
		return endpoint{}, fmt.Errorf("endpoint %q has empty node", raw)
	}
	if ifaceName == "" {
		return endpoint{}, fmt.Errorf("endpoint %q has empty interface", raw)
	}

	node := model.NodeID(nodeName)
	return endpoint{
		node:  node,
		iface: normalize(node, model.InterfaceID(ifaceName)),
	}, nil
}

type builder struct {
	normalize       InterfaceNormalizer
	nodes           map[model.NodeID]model.Node
	interfaces      map[interfaceKey]model.Interface
	links           map[linkKey]model.Link
	edgePorts       map[model.EdgePortID]model.EdgePort
	nodeConfigNames map[model.NodeID]string
}

func newBuilder(normalize InterfaceNormalizer) *builder {
	return &builder{
		normalize:       normalize,
		nodes:           map[model.NodeID]model.Node{},
		interfaces:      map[interfaceKey]model.Interface{},
		links:           map[linkKey]model.Link{},
		edgePorts:       map[model.EdgePortID]model.EdgePort{},
		nodeConfigNames: map[model.NodeID]string{},
	}
}

func (b *builder) addNode(id model.NodeID) {
	b.nodes[id] = model.Node{ID: id}
}

func (b *builder) hasNode(id model.NodeID) bool {
	_, ok := b.nodes[id]
	return ok
}

func (b *builder) addInterface(node model.NodeID, iface model.InterfaceID, vrf model.VRF) {
	b.interfaces[interfaceKey{node: node, iface: iface, vrf: vrf}] = model.Interface{
		Node: node,
		ID:   iface,
		VRF:  vrf,
	}
}

func (b *builder) addLink(nodeA model.NodeID, ifaceA model.InterfaceID, nodeB model.NodeID, ifaceB model.InterfaceID) {
	b.links[linkKey{nodeA: nodeA, ifaceA: ifaceA, nodeB: nodeB, ifaceB: ifaceB}] = model.Link{
		NodeA:      nodeA,
		InterfaceA: ifaceA,
		NodeB:      nodeB,
		InterfaceB: ifaceB,
	}
}

func (b *builder) addEdgePort(edge model.EdgePort) {
	b.edgePorts[edge.ID] = edge
}

func (b *builder) hasEdgePort(id model.EdgePortID) bool {
	_, ok := b.edgePorts[id]
	return ok
}

func (b *builder) addNodeConfigName(node model.NodeID, configName string) {
	b.nodeConfigNames[node] = configName
}

func (b *builder) build() Topology {
	nodes := valuesSorted(b.nodes, func(a, c model.Node) bool {
		return a.ID < c.ID
	})
	interfaces := valuesSorted(b.interfaces, func(a, c model.Interface) bool {
		if a.Node != c.Node {
			return a.Node < c.Node
		}
		if a.ID != c.ID {
			return a.ID < c.ID
		}
		return a.VRF < c.VRF
	})
	links := valuesSorted(b.links, func(a, c model.Link) bool {
		if a.NodeA != c.NodeA {
			return a.NodeA < c.NodeA
		}
		if a.InterfaceA != c.InterfaceA {
			return a.InterfaceA < c.InterfaceA
		}
		if a.NodeB != c.NodeB {
			return a.NodeB < c.NodeB
		}
		return a.InterfaceB < c.InterfaceB
	})
	edgePorts := valuesSorted(b.edgePorts, func(a, c model.EdgePort) bool {
		return a.ID < c.ID
	})

	return Topology{
		Nodes:           nodes,
		Interfaces:      interfaces,
		Links:           links,
		EdgePorts:       edgePorts,
		NodeConfigNames: cloneMap(b.nodeConfigNames),
	}
}

type interfaceKey struct {
	node  model.NodeID
	iface model.InterfaceID
	vrf   model.VRF
}

type linkKey struct {
	nodeA  model.NodeID
	ifaceA model.InterfaceID
	nodeB  model.NodeID
	ifaceB model.InterfaceID
}

func valuesSorted[M ~map[K]V, K comparable, V any](m M, less func(a, b V) bool) []V {
	values := make([]V, 0, len(m))
	for _, value := range m {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return less(values[i], values[j])
	})
	return values
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	clone := make(map[K]V, len(m))
	for key, value := range m {
		clone[key] = value
	}
	return clone
}
