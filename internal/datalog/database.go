package datalog

import (
	"fmt"
	"sort"
	"strings"
)

type Database struct {
	schema    Schema
	relations map[string]map[string]Row
}

func NewDatabase(schema Schema) (*Database, error) {
	if err := validateSchema(schema); err != nil {
		return nil, err
	}

	db := &Database{
		schema:    schema,
		relations: map[string]map[string]Row{},
	}
	for name := range schema.Relations {
		db.relations[name] = map[string]Row{}
	}
	return db, nil
}

func (db *Database) Schema() Schema {
	return db.schema
}

func (db *Database) Insert(relation string, row Row) error {
	decl, err := db.decl(relation)
	if err != nil {
		return err
	}
	if err := validateRow(decl, row); err != nil {
		return fmt.Errorf("insert %s: %w", relation, err)
	}
	db.relations[relation][rowKey(row)] = cloneRow(row)
	return nil
}

func (db *Database) Delete(relation string, row Row) error {
	decl, err := db.decl(relation)
	if err != nil {
		return err
	}
	if err := validateRow(decl, row); err != nil {
		return fmt.Errorf("delete %s: %w", relation, err)
	}
	delete(db.relations[relation], rowKey(row))
	return nil
}

func (db *Database) Query(relation string) ([]Row, error) {
	if _, err := db.decl(relation); err != nil {
		return nil, err
	}

	rows := make([]Row, 0, len(db.relations[relation]))
	for _, row := range db.relations[relation] {
		rows = append(rows, cloneRow(row))
	}
	sortRows(rows)
	return rows, nil
}

func (db *Database) Apply(delta Delta) error {
	for _, change := range delta.Deletes {
		if err := db.Delete(change.Relation, change.Row); err != nil {
			return err
		}
	}
	for _, change := range delta.Inserts {
		if err := db.Insert(change.Relation, change.Row); err != nil {
			return err
		}
	}
	return nil
}

func (db *Database) Clone() (*Database, error) {
	clone, err := NewDatabase(db.schema)
	if err != nil {
		return nil, err
	}
	for relation, rows := range db.relations {
		for _, row := range rows {
			clone.relations[relation][rowKey(row)] = cloneRow(row)
		}
	}
	return clone, nil
}

func (db *Database) Diff(other *Database) (Delta, error) {
	if err := sameSchema(db.schema, other.schema); err != nil {
		return Delta{}, err
	}

	var delta Delta
	for _, relation := range sortedRelationNames(db.schema) {
		left := db.relations[relation]
		right := other.relations[relation]
		for key, row := range left {
			if _, ok := right[key]; !ok {
				delta.Deletes = append(delta.Deletes, RowChange{Relation: relation, Row: cloneRow(row)})
			}
		}
		for key, row := range right {
			if _, ok := left[key]; !ok {
				delta.Inserts = append(delta.Inserts, RowChange{Relation: relation, Row: cloneRow(row)})
			}
		}
	}
	sortChanges(delta.Deletes)
	sortChanges(delta.Inserts)
	return delta, nil
}

func (db *Database) decl(relation string) (RelationDecl, error) {
	decl, ok := db.schema.Relations[relation]
	if !ok {
		return RelationDecl{}, fmt.Errorf("unknown relation %q", relation)
	}
	if _, ok := db.relations[relation]; !ok {
		db.relations[relation] = map[string]Row{}
	}
	return decl, nil
}

func validateSchema(schema Schema) error {
	if len(schema.Relations) == 0 {
		return fmt.Errorf("schema must declare at least one relation")
	}
	for name, decl := range schema.Relations {
		if name == "" || decl.Name == "" || name != decl.Name {
			return fmt.Errorf("invalid relation declaration %q", name)
		}
		if len(decl.Fields) == 0 {
			return fmt.Errorf("relation %q must declare at least one field", name)
		}
		seen := map[string]bool{}
		for _, field := range decl.Fields {
			if field.Name == "" {
				return fmt.Errorf("relation %q has an empty field name", name)
			}
			if seen[field.Name] {
				return fmt.Errorf("relation %q declares duplicate field %q", name, field.Name)
			}
			seen[field.Name] = true
			if !validKind(field.Kind) {
				return fmt.Errorf("relation %q field %q has unsupported type %q", name, field.Name, field.Kind)
			}
		}
	}
	return nil
}

func validateRow(decl RelationDecl, row Row) error {
	if len(row) != len(decl.Fields) {
		return fmt.Errorf("arity mismatch: got %d values, want %d", len(row), len(decl.Fields))
	}
	for i, value := range row {
		if value.Kind != decl.Fields[i].Kind {
			return fmt.Errorf("field %q has type %s, got %s", decl.Fields[i].Name, decl.Fields[i].Kind, value.Kind)
		}
	}
	return nil
}

func sameSchema(left, right Schema) error {
	if len(left.Relations) != len(right.Relations) {
		return fmt.Errorf("schema relation count mismatch")
	}
	for name, leftDecl := range left.Relations {
		rightDecl, ok := right.Relations[name]
		if !ok {
			return fmt.Errorf("schema missing relation %q", name)
		}
		if len(leftDecl.Fields) != len(rightDecl.Fields) {
			return fmt.Errorf("schema relation %q field count mismatch", name)
		}
		for i := range leftDecl.Fields {
			if leftDecl.Fields[i] != rightDecl.Fields[i] {
				return fmt.Errorf("schema relation %q field %d mismatch", name, i)
			}
		}
	}
	return nil
}

func validKind(kind ValueKind) bool {
	switch kind {
	case KindString, KindBool, KindAddr, KindPrefix:
		return true
	default:
		return false
	}
}

func cloneRow(row Row) Row {
	clone := make(Row, len(row))
	copy(clone, row)
	return clone
}

func rowKey(row Row) string {
	var key strings.Builder
	for _, value := range row {
		kind := string(value.Kind)
		data := value.StringValue()
		fmt.Fprintf(&key, "%d:%s%d:%s", len(kind), kind, len(data), data)
	}
	return key.String()
}

func sortRows(rows []Row) {
	sort.Slice(rows, func(i, j int) bool {
		return rowKey(rows[i]) < rowKey(rows[j])
	})
}

func sortedRelationNames(schema Schema) []string {
	names := make([]string, 0, len(schema.Relations))
	for name := range schema.Relations {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortChanges(changes []RowChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Relation != changes[j].Relation {
			return changes[i].Relation < changes[j].Relation
		}
		return rowKey(changes[i].Row) < rowKey(changes[j].Row)
	})
}
