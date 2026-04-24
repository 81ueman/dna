package datalog

import (
	"net/netip"
	"testing"
)

func testSchema() Schema {
	return Schema{Relations: map[string]RelationDecl{
		"Route": {
			Name: "Route",
			Fields: []Field{
				{Name: "node", Kind: KindString},
				{Name: "up", Kind: KindBool},
				{Name: "nextHop", Kind: KindAddr},
				{Name: "prefix", Kind: KindPrefix},
			},
		},
	}}
}

func testRow() Row {
	return NewRow(
		StringValue("r1"),
		BoolValue(true),
		AddrValue(netip.MustParseAddr("192.0.2.1")),
		PrefixValue(netip.MustParsePrefix("10.0.0.42/24")),
	)
}

func TestDatabaseInsertDeleteQuery(t *testing.T) {
	db, err := NewDatabase(testSchema())
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}

	if err := db.Insert("Route", testRow()); err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
	if err := db.Insert("Route", testRow()); err != nil {
		t.Fatalf("duplicate Insert returned error: %v", err)
	}

	rows, err := db.Query("Route")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got := rows[0][3].Prefix.String(); got != "10.0.0.0/24" {
		t.Fatalf("prefix = %s, want masked prefix", got)
	}

	if err := db.Delete("Route", testRow()); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	rows, err = db.Query("Route")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows after delete = %d, want 0", len(rows))
	}
}

func TestDatabaseRejectsTypeMismatch(t *testing.T) {
	db, err := NewDatabase(testSchema())
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}

	err = db.Insert("Route", NewRow(StringValue("r1")))
	if err == nil {
		t.Fatalf("Insert returned nil error for arity mismatch")
	}

	err = db.Insert("Route", NewRow(
		StringValue("r1"),
		StringValue("true"),
		AddrValue(netip.MustParseAddr("192.0.2.1")),
		PrefixValue(netip.MustParsePrefix("10.0.0.0/24")),
	))
	if err == nil {
		t.Fatalf("Insert returned nil error for type mismatch")
	}
}

func TestDatabaseDiffIsDeterministic(t *testing.T) {
	left, err := NewDatabase(testSchema())
	if err != nil {
		t.Fatalf("NewDatabase left: %v", err)
	}
	right, err := NewDatabase(testSchema())
	if err != nil {
		t.Fatalf("NewDatabase right: %v", err)
	}

	row1 := testRow()
	row2 := NewRow(
		StringValue("r2"),
		BoolValue(false),
		AddrValue(netip.MustParseAddr("192.0.2.2")),
		PrefixValue(netip.MustParsePrefix("10.0.1.0/24")),
	)
	if err := left.Insert("Route", row1); err != nil {
		t.Fatalf("left insert row1: %v", err)
	}
	if err := right.Insert("Route", row2); err != nil {
		t.Fatalf("right insert row2: %v", err)
	}

	delta, err := left.Diff(right)
	if err != nil {
		t.Fatalf("Diff returned error: %v", err)
	}
	if len(delta.Deletes) != 1 || len(delta.Inserts) != 1 {
		t.Fatalf("delta = %+v, want one delete and one insert", delta)
	}
	if got := delta.Deletes[0].Row[0].String; got != "r1" {
		t.Fatalf("delete row node = %q, want r1", got)
	}
	if got := delta.Inserts[0].Row[0].String; got != "r2" {
		t.Fatalf("insert row node = %q, want r2", got)
	}
}

func TestDatabaseRowKeysDistinguishEmbeddedNULStrings(t *testing.T) {
	schema := Schema{Relations: map[string]RelationDecl{
		"Pair": {
			Name: "Pair",
			Fields: []Field{
				{Name: "left", Kind: KindString},
				{Name: "right", Kind: KindString},
			},
		},
	}}
	db, err := NewDatabase(schema)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}

	row1 := NewRow(StringValue("x\x00string:y"), StringValue("z"))
	row2 := NewRow(StringValue("x"), StringValue("y\x00string:z"))
	if err := db.Insert("Pair", row1); err != nil {
		t.Fatalf("insert row1: %v", err)
	}
	if err := db.Insert("Pair", row2); err != nil {
		t.Fatalf("insert row2: %v", err)
	}

	rows, err := db.Query("Pair")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
}
