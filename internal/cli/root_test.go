package cli

import (
	"bytes"
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
	for _, want := range []string{"--topology", "--old-configs", "--new-configs"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff help does not mention %s; got:\n%s", want, got)
		}
	}
}
