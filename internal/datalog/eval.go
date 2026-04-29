package datalog

import (
	"fmt"
	"net/netip"
)

func Evaluate(program *Program, facts *Database) (*Database, error) {
	if err := sameSchema(program.Schema, facts.schema); err != nil {
		return nil, err
	}

	result, err := NewDatabase(program.Schema)
	if err != nil {
		return nil, err
	}
	derived := derivedRelations(program)
	for relation, rows := range facts.relations {
		if derived[relation] {
			continue
		}
		for _, row := range rows {
			if err := result.Insert(relation, row); err != nil {
				return nil, err
			}
		}
	}

	changed := true
	for changed {
		changed = false
		for _, rule := range program.Rules {
			rows, err := evalRule(program.Schema, result, rule)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				before := len(result.relations[rule.Head.Relation])
				if err := result.Insert(rule.Head.Relation, row); err != nil {
					return nil, err
				}
				if len(result.relations[rule.Head.Relation]) != before {
					changed = true
				}
			}
		}
	}

	return result, nil
}

func ApplyDelta(program *Program, current *Database, delta Delta) (*Database, error) {
	base, err := current.Clone()
	if err != nil {
		return nil, err
	}
	for relation := range derivedRelations(program) {
		base.relations[relation] = map[string]Row{}
	}
	if err := base.Apply(delta); err != nil {
		return nil, err
	}
	return Evaluate(program, base)
}

func evalRule(schema Schema, db *Database, rule Rule) ([]Row, error) {
	bindings := []map[string]Value{{}}
	for _, atom := range rule.Body {
		decl := schema.Relations[atom.Relation]
		rows, err := db.Query(atom.Relation)
		if err != nil {
			return nil, err
		}
		var next []map[string]Value
		for _, binding := range bindings {
			for _, row := range rows {
				joined, ok, err := matchAtom(decl, atom, row, binding)
				if err != nil {
					return nil, err
				}
				if ok {
					next = append(next, joined)
				}
			}
		}
		bindings = next
	}

	headDecl := schema.Relations[rule.Head.Relation]
	rows := make([]Row, 0, len(bindings))
	for _, binding := range bindings {
		row, err := buildHeadRow(headDecl, rule.Head, binding)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sortRows(rows)
	return rows, nil
}

func matchAtom(decl RelationDecl, atom Atom, row Row, binding map[string]Value) (map[string]Value, bool, error) {
	next := cloneBinding(binding)
	for i, term := range atom.Terms {
		value := row[i]
		if term.IsVariable() {
			bound, ok := next[term.Variable]
			if ok {
				if rowKey(Row{bound}) != rowKey(Row{value}) {
					return nil, false, nil
				}
			} else {
				next[term.Variable] = value
			}
			continue
		}
		litValue, err := literalValue(term.Literal, decl.Fields[i].Kind)
		if err != nil {
			return nil, false, err
		}
		if rowKey(Row{litValue}) != rowKey(Row{value}) {
			return nil, false, nil
		}
	}
	return next, true, nil
}

func buildHeadRow(decl RelationDecl, head Atom, binding map[string]Value) (Row, error) {
	row := make(Row, len(head.Terms))
	for i, term := range head.Terms {
		if term.IsVariable() {
			value, ok := binding[term.Variable]
			if !ok {
				return nil, fmt.Errorf("unbound variable %q", term.Variable)
			}
			row[i] = value
			continue
		}
		value, err := literalValue(term.Literal, decl.Fields[i].Kind)
		if err != nil {
			return nil, err
		}
		row[i] = value
	}
	return row, nil
}

func literalValue(lit Literal, kind ValueKind) (Value, error) {
	if err := literalMatches(lit, kind); err != nil {
		return Value{}, err
	}
	switch kind {
	case KindString:
		return StringValue(lit.String), nil
	case KindBool:
		return BoolValue(lit.Bool), nil
	case KindAddr:
		addr, err := netip.ParseAddr(lit.String)
		if err != nil {
			return Value{}, err
		}
		return AddrValue(addr), nil
	case KindPrefix:
		prefix, err := netip.ParsePrefix(lit.String)
		if err != nil {
			return Value{}, err
		}
		return PrefixValue(prefix), nil
	default:
		return Value{}, fmt.Errorf("unsupported type %s", kind)
	}
}

func derivedRelations(program *Program) map[string]bool {
	derived := map[string]bool{}
	for _, rule := range program.Rules {
		derived[rule.Head.Relation] = true
	}
	return derived
}

func cloneBinding(binding map[string]Value) map[string]Value {
	clone := make(map[string]Value, len(binding))
	for key, value := range binding {
		clone[key] = value
	}
	return clone
}
