package datalog

import (
	"strings"
	"testing"
)

func TestParseProgram(t *testing.T) {
	src := []byte(`
		# static routes become forwarding rows
		relation StaticRoute(node: string, vrf: string, prefix: prefix, nextHop: addr).
		relation Forward(node: string, vrf: string, prefix: prefix, action: string, nextHop: addr, iface: string).

		Forward(node, vrf, prefix, "next-hop", nextHop, "") :-
			StaticRoute(node, vrf, prefix, nextHop).
	`)

	program, err := ParseProgram("rules.dna", src)
	if err != nil {
		t.Fatalf("ParseProgram returned error: %v", err)
	}

	if len(program.Schema.Relations) != 2 {
		t.Fatalf("relations = %d, want 2", len(program.Schema.Relations))
	}
	if len(program.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(program.Rules))
	}
	if got := program.Rules[0].Head.Relation; got != "Forward" {
		t.Fatalf("head relation = %q, want Forward", got)
	}
}

func TestParseProgramAllowsNegationTextInsideStringLiterals(t *testing.T) {
	src := []byte(`
		relation Input(node: string).
		relation Action(node: string, label: string, note: string).

		Action(node, "drop!", "do not alarm") :-
			Input(node).
	`)

	program, err := ParseProgram("rules.dna", src)
	if err != nil {
		t.Fatalf("ParseProgram returned error: %v", err)
	}
	if len(program.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(program.Rules))
	}
}

func TestParseProgramCutsRuleSeparatorOutsideStringLiterals(t *testing.T) {
	src := []byte(`
		relation Input(node: string).
		relation Action(node: string, label: string).

		Action(node, "x:-y") :-
			Input(node).
	`)

	program, err := ParseProgram("rules.dna", src)
	if err != nil {
		t.Fatalf("ParseProgram returned error: %v", err)
	}
	if got := program.Rules[0].Head.Terms[1].Literal.String; got != "x:-y" {
		t.Fatalf("head literal = %q, want x:-y", got)
	}
}

func TestParseProgramRejectsUnsupportedFeatures(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "negation",
			src: `
				relation A(x: string).
				relation B(x: string).
				B(x) :- not A(x).
			`,
			want: "unsupported negation",
		},
		{
			name: "aggregation",
			src: `
				relation A(x: string).
				relation B(x: string).
				B(x) :- Aggregate(A(x)).
			`,
			want: "unsupported aggregation",
		},
		{
			name: "recursion",
			src: `
				relation A(x: string).
				A(x) :- A(x).
			`,
			want: "unsupported recursive rule",
		},
		{
			name: "unknown relation",
			src: `
				relation A(x: string).
				A(x) :- Missing(x).
			`,
			want: "unknown relation",
		},
		{
			name: "arity mismatch",
			src: `
				relation A(x: string).
				relation B(x: string).
				B(x) :- A(x, y).
			`,
			want: "arity mismatch",
		},
		{
			name: "type mismatch",
			src: `
				relation A(x: string).
				relation B(x: bool).
				B(x) :- A(x).
			`,
			want: "target field requires bool",
		},
		{
			name: "anonymous variable",
			src: `
				relation A(x: string).
				relation B(x: string).
				B(x) :- A(_).
			`,
			want: "unsupported anonymous variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProgram("bad.dna", []byte(tt.src))
			if err == nil {
				t.Fatalf("ParseProgram returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
