package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/81ueman/dna/internal/forwarding"
	"github.com/81ueman/dna/internal/model"
	"github.com/81ueman/dna/internal/topology"
)

type BatfishParser struct {
	UV          string
	ProjectDir  string
	Host        string
	Network     string
	Snapshot    string
	ExtraEnv    []string
	ExporterBin string
}

func (p BatfishParser) LoadSnapshotDir(ctx context.Context, path string, topo topology.Topology) (Snapshot, error) {
	output, err := os.CreateTemp("", "dna-batfish-export-*.json")
	if err != nil {
		return Snapshot{}, fmt.Errorf("create batfish export temp file: %w", err)
	}
	outputPath := output.Name()
	if err := output.Close(); err != nil {
		return Snapshot{}, fmt.Errorf("close batfish export temp file: %w", err)
	}
	defer func() {
		_ = os.Remove(outputPath)
	}()

	uv := p.UV
	if uv == "" {
		uv = "uv"
	}
	projectDir := p.ProjectDir
	if projectDir == "" {
		projectDir, err = defaultBatfishExporterProjectDir()
		if err != nil {
			return Snapshot{}, err
		}
	}
	host := p.Host
	if host == "" {
		host = "localhost"
	}
	network := p.Network
	if network == "" {
		network = "dna"
	}
	snapshot := p.Snapshot
	if snapshot == "" {
		snapshot = "snapshot"
	}
	exporterBin := p.ExporterBin
	if exporterBin == "" {
		exporterBin = "batfish-export"
	}

	args := []string{
		"run",
		"--project", projectDir,
		exporterBin,
		"--snapshot", path,
		"--output", outputPath,
		"--host", host,
		"--network", network,
		"--snapshot-name", snapshot,
	}
	cmd := exec.CommandContext(ctx, uv, args...)
	cmd.Env = append(os.Environ(), p.ExtraEnv...)
	combined, err := cmd.CombinedOutput()
	if err != nil {
		return Snapshot{}, fmt.Errorf("run batfish exporter: %w: %s", err, combined)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read batfish export: %w", err)
	}
	snapshotFacts, err := ParseBatfishExport(data, topo)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse batfish export: %w", err)
	}
	return snapshotFacts, nil
}

func ParseBatfishExport(data []byte, topo topology.Topology) (Snapshot, error) {
	var input batfishExport
	if err := json.Unmarshal(data, &input); err != nil {
		return Snapshot{}, fmt.Errorf("parse JSON: %w", err)
	}

	validator, err := newValidator(topo)
	if err != nil {
		return Snapshot{}, err
	}

	seenNodes := map[model.NodeID]bool{}
	for i, rawNode := range input.Nodes {
		if rawNode == "" {
			return Snapshot{}, fmt.Errorf("node %d is empty", i)
		}
		node, ok := validator.resolveNode(rawNode)
		if !ok {
			return Snapshot{}, fmt.Errorf("node %q not found in topology", rawNode)
		}
		seenNodes[node] = true
	}

	var snapshot Snapshot
	seenStates := map[interfaceStateKey]bool{}
	seenAddresses := map[model.InterfaceAddress]bool{}
	for i, iface := range input.Interfaces {
		node, ok := validator.resolveNode(iface.Node)
		if !ok {
			return Snapshot{}, fmt.Errorf("interface %d node %q not found in topology", i, iface.Node)
		}
		seenNodes[node] = true
		if iface.Interface == "" {
			return Snapshot{}, fmt.Errorf("interface %d name must not be empty", i)
		}
		interfaceID := model.InterfaceID(iface.Interface)
		vrf := defaultVRF(iface.VRF)
		if !validator.hasInterface(node, interfaceID, vrf) {
			return Snapshot{}, fmt.Errorf("interface %q in VRF %q on node %q not found in topology", interfaceID, vrf, node)
		}

		state := model.InterfaceState{Node: node, Interface: interfaceID, Up: iface.Up}
		stateKey := interfaceStateKey{node: state.Node, iface: state.Interface}
		if !seenStates[stateKey] {
			seenStates[stateKey] = true
			snapshot.InterfaceStates = append(snapshot.InterfaceStates, state)
		}

		for j, rawPrefix := range iface.Addresses {
			prefix, err := parsePrefix(rawPrefix)
			if err != nil {
				return Snapshot{}, fmt.Errorf("interface %d address %d %q: %w", i, j, rawPrefix, err)
			}
			address := model.InterfaceAddress{
				Node:      node,
				Interface: interfaceID,
				VRF:       vrf,
				Prefix:    prefix,
			}
			if seenAddresses[address] {
				continue
			}
			seenAddresses[address] = true
			snapshot.InterfaceAddresses = append(snapshot.InterfaceAddresses, address)
		}
	}

	seenRoutes := map[model.StaticRoute]bool{}
	for i, route := range input.StaticRoutes {
		node, ok := validator.resolveNode(route.Node)
		if !ok {
			return Snapshot{}, fmt.Errorf("static route %d node %q not found in topology", i, route.Node)
		}
		seenNodes[node] = true
		vrf := defaultVRF(route.VRF)
		if !validator.hasVRF(node, vrf) {
			return Snapshot{}, fmt.Errorf("VRF %q on node %q not found in topology", vrf, node)
		}
		prefix, err := parsePrefix(route.Prefix)
		if err != nil {
			return Snapshot{}, fmt.Errorf("static route %d prefix %q: %w", i, route.Prefix, err)
		}

		hasNextHop := route.NextHop != ""
		if route.NextHopInterface != "" && !route.Drop && !hasNextHop {
			return Snapshot{}, fmt.Errorf(
				"static route %d on node %q prefix %s uses unsupported interface next-hop %q",
				i,
				node,
				prefix,
				route.NextHopInterface,
			)
		}
		if hasNextHop == route.Drop {
			return Snapshot{}, fmt.Errorf("static route %d must specify exactly one of next_hop or drop", i)
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
				return Snapshot{}, fmt.Errorf("static route %d next_hop %q: %w", i, route.NextHop, err)
			}
			staticRoute.Action = model.StaticRouteActionNextHop
			staticRoute.NextHop = nextHop
		}
		if seenRoutes[staticRoute] {
			continue
		}
		seenRoutes[staticRoute] = true
		snapshot.StaticRoutes = append(snapshot.StaticRoutes, staticRoute)
	}

	for _, node := range topo.Nodes {
		if !seenNodes[node.ID] {
			return Snapshot{}, fmt.Errorf("snapshot missing config for topology node %q", node.ID)
		}
	}

	snapshot.ConnectedRoutes = forwarding.ConnectedRoutes(snapshot.InterfaceAddresses, snapshot.InterfaceStates)
	snapshot.sort()
	return snapshot, nil
}

type batfishExport struct {
	Nodes        []string             `json:"nodes"`
	Interfaces   []batfishInterface   `json:"interfaces"`
	StaticRoutes []batfishStaticRoute `json:"static_routes"`
}

type batfishInterface struct {
	Node      string   `json:"node"`
	Interface string   `json:"interface"`
	VRF       string   `json:"vrf"`
	Addresses []string `json:"addresses"`
	Up        bool     `json:"up"`
}

type batfishStaticRoute struct {
	Node             string `json:"node"`
	VRF              string `json:"vrf"`
	Prefix           string `json:"prefix"`
	NextHop          string `json:"next_hop,omitempty"`
	NextHopInterface string `json:"next_hop_interface,omitempty"`
	Drop             bool   `json:"drop,omitempty"`
}

type interfaceStateKey struct {
	node  model.NodeID
	iface model.InterfaceID
}

func defaultBatfishExporterProjectDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find batfish exporter project: %w", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "tools", "batfish-exporter")
		if _, err := os.Stat(filepath.Join(candidate, "pyproject.toml")); err == nil {
			return candidate, nil
		}
		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return "", fmt.Errorf("find batfish exporter project: tools/batfish-exporter not found from %q", cwd)
}
