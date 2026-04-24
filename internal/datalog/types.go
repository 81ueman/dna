package datalog

import (
	"fmt"
	"net/netip"
)

type ValueKind string

const (
	KindString ValueKind = "string"
	KindBool   ValueKind = "bool"
	KindAddr   ValueKind = "addr"
	KindPrefix ValueKind = "prefix"
)

type Field struct {
	Name string
	Kind ValueKind
}

type RelationDecl struct {
	Name   string
	Fields []Field
}

type Schema struct {
	Relations map[string]RelationDecl
}

type Program struct {
	Schema Schema
	Rules  []Rule
}

type Rule struct {
	Head Atom
	Body []Atom
}

type Atom struct {
	Relation string
	Terms    []Term
}

type Term struct {
	Variable string
	Literal  Literal
}

func Var(name string) Term {
	return Term{Variable: name}
}

func StringLit(value string) Term {
	return Term{Literal: Literal{Kind: KindString, String: value, Set: true}}
}

func BoolLit(value bool) Term {
	return Term{Literal: Literal{Kind: KindBool, Bool: value, Set: true}}
}

func (t Term) IsVariable() bool {
	return t.Variable != ""
}

type Literal struct {
	Kind   ValueKind
	String string
	Bool   bool
	Set    bool
}

type Value struct {
	Kind   ValueKind
	String string
	Bool   bool
	Addr   netip.Addr
	Prefix netip.Prefix
}

func StringValue(value string) Value {
	return Value{Kind: KindString, String: value}
}

func BoolValue(value bool) Value {
	return Value{Kind: KindBool, Bool: value}
}

func AddrValue(value netip.Addr) Value {
	return Value{Kind: KindAddr, Addr: value}
}

func PrefixValue(value netip.Prefix) Value {
	return Value{Kind: KindPrefix, Prefix: value.Masked()}
}

func (v Value) StringValue() string {
	switch v.Kind {
	case KindString:
		return v.String
	case KindBool:
		return fmt.Sprintf("%t", v.Bool)
	case KindAddr:
		return v.Addr.String()
	case KindPrefix:
		return v.Prefix.String()
	default:
		return "<invalid>"
	}
}

type Row []Value

func NewRow(values ...Value) Row {
	row := make(Row, len(values))
	copy(row, values)
	return row
}

type RowChange struct {
	Relation string
	Row      Row
}

type Delta struct {
	Inserts []RowChange
	Deletes []RowChange
}
