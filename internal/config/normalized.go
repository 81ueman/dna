package config

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/topology"
	"gopkg.in/yaml.v3"
)

type Snapshot struct {
	InterfaceAddresses []model.InterfaceAddress
	InterfaceStates    []model.InterfaceState
	ConnectedRoutes    []model.ConnectedRoute
	StaticRoutes       []model.StaticRoute
}

func LoadSnapshotDir(path string, topo topology.Topology) (Snapshot, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot directory: %w", err)
	}

	validator := newValidator(topo)
	seenNodes := map[model.NodeID]string{}
	var snapshot Snapshot

	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}

		filePath := filepath.Join(path, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return Snapshot{}, fmt.Errorf("read node config %q: %w", entry.Name(), err)
		}

		node, nodeSnapshot, err := parseNodeConfig(data, topo)
		if err != nil {
			return Snapshot{}, fmt.Errorf("parse node config %q: %w", entry.Name(), err)
		}
		if firstFile, ok := seenNodes[node]; ok {
			return Snapshot{}, fmt.Errorf("duplicate node %q in %q and %q", node, firstFile, entry.Name())
		}
		if !validator.hasNode(node) {
			return Snapshot{}, fmt.Errorf("node %q not found in topology", node)
		}
		seenNodes[node] = entry.Name()

		snapshot.append(nodeSnapshot)
	}

	for _, node := range topo.Nodes {
		if _, ok := seenNodes[node.ID]; !ok {
			return Snapshot{}, fmt.Errorf("snapshot missing config for topology node %q", node.ID)
		}
	}

	snapshot.sort()
	return snapshot, nil
}

func ParseNodeConfig(data []byte, topo topology.Topology) (Snapshot, error) {
	_, snapshot, err := parseNodeConfig(data, topo)
	return snapshot, err
}

func parseNodeConfig(data []byte, topo topology.Topology) (model.NodeID, Snapshot, error) {
	var input nodeConfigYAML
	if err := yaml.Unmarshal(data, &input); err != nil {
		return "", Snapshot{}, fmt.Errorf("parse normalized config: %w", err)
	}
	if input.Node == "" {
		return "", Snapshot{}, fmt.Errorf("node is required")
	}

	validator := newValidator(topo)
	node, ok := validator.resolveNode(input.Node)
	if !ok {
		return "", Snapshot{}, fmt.Errorf("node %q not found in topology", input.Node)
	}

	var snapshot Snapshot
	for ifaceName, iface := range input.Interfaces {
		if ifaceName == "" {
			return "", Snapshot{}, fmt.Errorf("interface name must not be empty")
		}

		interfaceID := model.InterfaceID(ifaceName)
		vrf := defaultVRF(iface.VRF)
		if !validator.hasInterface(node, interfaceID, vrf) {
			return "", Snapshot{}, fmt.Errorf("interface %q in VRF %q on node %q not found in topology", interfaceID, vrf, node)
		}

		up := true
		if iface.Up != nil {
			up = *iface.Up
		}
		snapshot.InterfaceStates = append(snapshot.InterfaceStates, model.InterfaceState{
			Node:      node,
			Interface: interfaceID,
			Up:        up,
		})

		for _, rawPrefix := range iface.Addresses {
			prefix, err := parsePrefix(rawPrefix)
			if err != nil {
				return "", Snapshot{}, fmt.Errorf("interface %q address %q: %w", interfaceID, rawPrefix, err)
			}
			snapshot.InterfaceAddresses = append(snapshot.InterfaceAddresses, model.InterfaceAddress{
				Node:      node,
				Interface: interfaceID,
				VRF:       vrf,
				Prefix:    prefix,
			})
			snapshot.ConnectedRoutes = append(snapshot.ConnectedRoutes, model.ConnectedRoute{
				Node:      node,
				VRF:       vrf,
				Prefix:    prefix,
				Interface: interfaceID,
			})
		}
	}

	for i, route := range input.StaticRoutes {
		vrf := defaultVRF(route.VRF)
		if !validator.hasVRF(node, vrf) {
			return "", Snapshot{}, fmt.Errorf("VRF %q on node %q not found in topology", vrf, node)
		}
		prefix, err := parsePrefix(route.Prefix)
		if err != nil {
			return "", Snapshot{}, fmt.Errorf("static route %d prefix %q: %w", i, route.Prefix, err)
		}

		hasNextHop := route.NextHop != ""
		if hasNextHop == route.Drop {
			return "", Snapshot{}, fmt.Errorf("static route %d must specify exactly one of next_hop or drop", i)
		}

		staticRoute := model.StaticRoute{
			Node:   node,
			VRF:    vrf,
			Prefix: prefix,
		}
		if route.Drop {
			staticRoute.Action = model.StaticRouteActionDrop
		} else {
			nextHop, err := netip.ParseAddr(route.NextHop)
			if err != nil {
				return "", Snapshot{}, fmt.Errorf("static route %d next_hop %q: %w", i, route.NextHop, err)
			}
			staticRoute.Action = model.StaticRouteActionNextHop
			staticRoute.NextHop = nextHop
		}
		snapshot.StaticRoutes = append(snapshot.StaticRoutes, staticRoute)
	}

	snapshot.sort()
	return node, snapshot, nil
}

type nodeConfigYAML struct {
	Node         string                   `yaml:"node"`
	Interfaces   map[string]interfaceYAML `yaml:"interfaces"`
	StaticRoutes []staticRouteYAML        `yaml:"static_routes"`
}

type interfaceYAML struct {
	VRF       string   `yaml:"vrf"`
	Addresses []string `yaml:"addresses"`
	Up        *bool    `yaml:"up"`
}

type staticRouteYAML struct {
	VRF     string `yaml:"vrf"`
	Prefix  string `yaml:"prefix"`
	NextHop string `yaml:"next_hop"`
	Drop    bool   `yaml:"drop"`
}

func defaultVRF(vrf string) model.VRF {
	if vrf == "" {
		return model.DefaultVRF
	}
	return model.VRF(vrf)
}

func parsePrefix(raw string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		return netip.Prefix{}, err
	}
	return prefix.Masked(), nil
}

func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

func (s *Snapshot) append(other Snapshot) {
	s.InterfaceAddresses = append(s.InterfaceAddresses, other.InterfaceAddresses...)
	s.InterfaceStates = append(s.InterfaceStates, other.InterfaceStates...)
	s.ConnectedRoutes = append(s.ConnectedRoutes, other.ConnectedRoutes...)
	s.StaticRoutes = append(s.StaticRoutes, other.StaticRoutes...)
}

func (s *Snapshot) sort() {
	sort.Slice(s.InterfaceAddresses, func(i, j int) bool {
		a, b := s.InterfaceAddresses[i], s.InterfaceAddresses[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.Interface != b.Interface {
			return a.Interface < b.Interface
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		return a.Prefix.String() < b.Prefix.String()
	})
	sort.Slice(s.InterfaceStates, func(i, j int) bool {
		a, b := s.InterfaceStates[i], s.InterfaceStates[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		return a.Interface < b.Interface
	})
	sort.Slice(s.ConnectedRoutes, func(i, j int) bool {
		a, b := s.ConnectedRoutes[i], s.ConnectedRoutes[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		if a.Prefix != b.Prefix {
			return a.Prefix.String() < b.Prefix.String()
		}
		return a.Interface < b.Interface
	})
	sort.Slice(s.StaticRoutes, func(i, j int) bool {
		a, b := s.StaticRoutes[i], s.StaticRoutes[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.VRF != b.VRF {
			return a.VRF < b.VRF
		}
		if a.Prefix != b.Prefix {
			return a.Prefix.String() < b.Prefix.String()
		}
		if a.Action != b.Action {
			return a.Action < b.Action
		}
		return a.NextHop.String() < b.NextHop.String()
	})
}

type validator struct {
	nodes       map[model.NodeID]bool
	configNames map[string]model.NodeID
	interfaces  map[interfaceKey]bool
	vrfs        map[vrfKey]bool
}

func newValidator(topo topology.Topology) validator {
	v := validator{
		nodes:       map[model.NodeID]bool{},
		configNames: map[string]model.NodeID{},
		interfaces:  map[interfaceKey]bool{},
		vrfs:        map[vrfKey]bool{},
	}
	for _, node := range topo.Nodes {
		v.nodes[node.ID] = true
		v.configNames[string(node.ID)] = node.ID
	}
	for node, configName := range topo.NodeConfigNames {
		if configName != "" {
			v.configNames[configName] = node
		}
	}
	for _, iface := range topo.Interfaces {
		v.interfaces[interfaceKey{node: iface.Node, iface: iface.ID, vrf: iface.VRF}] = true
		v.vrfs[vrfKey{node: iface.Node, vrf: iface.VRF}] = true
	}
	return v
}

func (v validator) resolveNode(name string) (model.NodeID, bool) {
	node, ok := v.configNames[name]
	return node, ok
}

func (v validator) hasNode(node model.NodeID) bool {
	return v.nodes[node]
}

func (v validator) hasInterface(node model.NodeID, iface model.InterfaceID, vrf model.VRF) bool {
	return v.interfaces[interfaceKey{node: node, iface: iface, vrf: vrf}]
}

func (v validator) hasVRF(node model.NodeID, vrf model.VRF) bool {
	return v.vrfs[vrfKey{node: node, vrf: vrf}]
}

type interfaceKey struct {
	node  model.NodeID
	iface model.InterfaceID
	vrf   model.VRF
}

type vrfKey struct {
	node model.NodeID
	vrf  model.VRF
}
