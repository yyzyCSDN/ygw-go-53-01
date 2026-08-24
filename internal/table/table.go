// Package table implements feature table definitions and the schema registry
// that version managers and importers consult for field layouts.
package table

import (
	"fmt"
	"strings"

	"featurestore/internal/model"
)

// Table describes a feature table: its stable identifier, display name and
// the ordered field schema of every version.
type Table struct {
	ID     string
	Name   string
	Fields []model.Field
}

// FieldIndex returns the position of a field name or -1 when absent.
func (t *Table) FieldIndex(name string) int {
	for i, field := range t.Fields {
		if field.Name == name {
			return i
		}
	}
	return -1
}

// HasField reports whether the table schema contains the given field.
func (t *Table) HasField(name string) bool {
	return t.FieldIndex(name) >= 0
}

// FieldNames returns the ordered field names of the table.
func (t *Table) FieldNames() []string {
	names := make([]string, 0, len(t.Fields))
	for _, field := range t.Fields {
		names = append(names, field.Name)
	}
	return names
}

// Validate checks table identity and schema consistency.
func (t *Table) Validate() error {
	if t == nil || strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("table id must not be empty")
	}
	seen := map[string]bool{}
	for _, field := range t.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return fmt.Errorf("table %s has an unnamed field", t.ID)
		}
		if seen[name] {
			return fmt.Errorf("table %s defines duplicate field %q", t.ID, name)
		}
		seen[name] = true
		switch field.Type {
		case model.FieldString, model.FieldInt, model.FieldFloat, model.FieldBool:
		default:
			return fmt.Errorf("table %s field %s has unknown type %q", t.ID, name, field.Type)
		}
	}
	return nil
}
