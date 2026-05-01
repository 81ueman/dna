package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "dna") {
		t.Fatalf("root help does not mention command name; got:\n%s", got)
	}
	if !strings.Contains(got, "diff") {
		t.Fatalf("root help does not mention diff subcommand; got:\n%s", got)
	}
}

func TestDiffHelp(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"diff", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff help returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"--topology", "--old-configs", "--new-configs", "--parser-backend"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff help does not mention %s; got:\n%s", want, got)
		}
	}
}

func TestDiffCommandReportsReachabilityChanges(t *testing.T) {
	fixture := writeStaticFixture(t)
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"diff",
		"--topology", fixture.topology,
		"--old-configs", fixture.oldConfigs,
		"--new-configs", fixture.newConfigs,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff returned error: %v\noutput:\n%s", err, out.String())
	}

	const want = "+Reach(h1,h2,default,10.0.2.0/24)\n"
	if got := out.String(); got != want {
		t.Fatalf("diff output = %q, want %q", got, want)
	}
}

func TestDiffCommandReportsNoReachabilityChanges(t *testing.T) {
	fixture := writeStaticFixture(t)
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"diff",
		"--topology", fixture.topology,
		"--old-configs", fixture.oldConfigs,
		"--new-configs", fixture.oldConfigs,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff returned error: %v\noutput:\n%s", err, out.String())
	}

	const want = "No reachability changes.\n"
	if got := out.String(); got != want {
		t.Fatalf("diff output = %q, want %q", got, want)
	}
}

func TestDiffCommandValidatesRequiredFlags(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"diff"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("diff succeeded, want required flag error")
	}
	if !strings.Contains(err.Error(), "--topology is required") {
		t.Fatalf("error = %q, want missing topology", err)
	}
}

func TestDiffCommandRejectsUnimplementedBatfishBackend(t *testing.T) {
	fixture := writeStaticFixture(t)
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"diff",
		"--topology", fixture.topology,
		"--old-configs", fixture.oldConfigs,
		"--new-configs", fixture.newConfigs,
		"--parser-backend", "batfish",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("diff succeeded, want batfish error")
	}
	if !strings.Contains(err.Error(), `parser backend "batfish" is not implemented yet`) {
		t.Fatalf("error = %q, want batfish not implemented", err)
	}
}

type staticFixture struct {
	topology   string
	oldConfigs string
	newConfigs string
}

func writeStaticFixture(t *testing.T) staticFixture {
	t.Helper()

	dir := t.TempDir()
	topologyPath := filepath.Join(dir, "topology.clab.yaml")
	oldDir := filepath.Join(dir, "old")
	newDir := filepath.Join(dir, "new")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("create old dir: %v", err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("create new dir: %v", err)
	}

	writeFile(t, topologyPath, `
name: static
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
    - name: h2
      node: r2
      interface: eth2
`)
	writeFile(t, filepath.Join(oldDir, "r1.yaml"), `
node: r1
interfaces:
  eth1:
    addresses:
      - 10.0.12.1/30
  eth2:
    addresses:
      - 10.0.1.1/24
`)
	writeFile(t, filepath.Join(oldDir, "r2.yaml"), `
node: r2
interfaces:
  eth1:
    addresses:
      - 10.0.12.2/30
  eth2:
    addresses:
      - 10.0.2.1/24
`)
	writeFile(t, filepath.Join(newDir, "r1.yaml"), `
node: r1
interfaces:
  eth1:
    addresses:
      - 10.0.12.1/30
  eth2:
    addresses:
      - 10.0.1.1/24
static_routes:
  - prefix: 10.0.2.0/24
    next_hop: 10.0.12.2
`)
	writeFile(t, filepath.Join(newDir, "r2.yaml"), `
node: r2
interfaces:
  eth1:
    addresses:
      - 10.0.12.2/30
  eth2:
    addresses:
      - 10.0.2.1/24
`)

	return staticFixture{
		topology:   topologyPath,
		oldConfigs: oldDir,
		newConfigs: newDir,
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
