package datalog

import (
	"net/netip"
	"testing"
)

func parseEvalProgram(t *testing.T) *Program {
	t.Helper()

	program, err := ParseProgram("rules.dna", []byte(`
		relation StaticRoute(node: string, vrf: string, prefix: prefix, nextHop: addr).
		relation InterfaceAddress(node: string, vrf: string, nextHop: addr, iface: string).
		relation Forward(node: string, vrf: string, prefix: prefix, action: string, nextHop: addr, iface: string).

		Forward(node, vrf, prefix, "next-hop", nextHop, "") :-
			StaticRoute(node, vrf, prefix, nextHop).

		Forward(node, vrf, prefix, "interface", nextHop, iface) :-
			StaticRoute(node, vrf, prefix, nextHop),
			InterfaceAddress(node, vrf, nextHop, iface).
	`))
	if err != nil {
		t.Fatalf("ParseProgram returned error: %v", err)
	}
	return program
}

func TestEvaluateStaticRouteProjection(t *testing.T) {
	program := parseEvalProgram(t)
	facts, err := NewDatabase(program.Schema)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	staticRoute := NewRow(
		StringValue("r1"),
		StringValue("default"),
		PrefixValue(netip.MustParsePrefix("10.0.0.0/24")),
		AddrValue(netip.MustParseAddr("192.0.2.1")),
	)
	if err := facts.Insert("StaticRoute", staticRoute); err != nil {
		t.Fatalf("insert StaticRoute: %v", err)
	}

	result, err := Evaluate(program, facts)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	rows, err := result.Query("Forward")
	if err != nil {
		t.Fatalf("Query Forward: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Forward rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row[0].String != "r1" || row[1].String != "default" || row[3].String != "next-hop" || row[5].String != "" {
		t.Fatalf("unexpected Forward row: %#v", row)
	}
}

func TestEvaluatePositiveJoin(t *testing.T) {
	program := parseEvalProgram(t)
	facts, err := NewDatabase(program.Schema)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	nextHop := netip.MustParseAddr("192.0.2.1")
	if err := facts.Insert("StaticRoute", NewRow(
		StringValue("r1"),
		StringValue("default"),
		PrefixValue(netip.MustParsePrefix("10.0.0.0/24")),
		AddrValue(nextHop),
	)); err != nil {
		t.Fatalf("insert StaticRoute: %v", err)
	}
	if err := facts.Insert("InterfaceAddress", NewRow(
		StringValue("r1"),
		StringValue("default"),
		AddrValue(nextHop),
		StringValue("Ethernet1"),
	)); err != nil {
		t.Fatalf("insert InterfaceAddress: %v", err)
	}

	result, err := Evaluate(program, facts)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	rows, err := result.Query("Forward")
	if err != nil {
		t.Fatalf("Query Forward: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("Forward rows = %d, want 2", len(rows))
	}
	actions := map[string]bool{}
	for _, row := range rows {
		actions[row[3].String] = true
	}
	if !actions["interface"] || !actions["next-hop"] {
		t.Fatalf("actions = %#v, want interface and next-hop", actions)
	}
}

func TestApplyDeltaMatchesFullRecompute(t *testing.T) {
	program := parseEvalProgram(t)
	facts, err := NewDatabase(program.Schema)
	if err != nil {
		t.Fatalf("NewDatabase facts: %v", err)
	}
	row := NewRow(
		StringValue("r1"),
		StringValue("default"),
		PrefixValue(netip.MustParsePrefix("10.0.0.0/24")),
		AddrValue(netip.MustParseAddr("192.0.2.1")),
	)
	if err := facts.Insert("StaticRoute", row); err != nil {
		t.Fatalf("insert StaticRoute: %v", err)
	}
	current, err := Evaluate(program, facts)
	if err != nil {
		t.Fatalf("Evaluate current: %v", err)
	}

	delta := Delta{Deletes: []RowChange{{Relation: "StaticRoute", Row: row}}}
	incremental, err := ApplyDelta(program, current, delta)
	if err != nil {
		t.Fatalf("ApplyDelta returned error: %v", err)
	}

	updatedFacts, err := NewDatabase(program.Schema)
	if err != nil {
		t.Fatalf("NewDatabase updatedFacts: %v", err)
	}
	full, err := Evaluate(program, updatedFacts)
	if err != nil {
		t.Fatalf("Evaluate full: %v", err)
	}

	diff, err := incremental.Diff(full)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if len(diff.Inserts) != 0 || len(diff.Deletes) != 0 {
		t.Fatalf("incremental/full diff = %+v, want empty", diff)
	}
}
