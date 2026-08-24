package ingest

import (
	"featurestore/internal/model"
	"featurestore/internal/table"
)

// fieldWindow computes the ordered list of fields an import segment is
// allowed to overwrite. The window is the intersection of the import file's
// fields and the live table schema: schema fields added online while the
// import is running are never part of the file and must be preserved.
func fieldWindow(t *table.Table, rows []Row) []string {
	file := fileFields(rows)
	known := schemaFields(t)
	knownSet := make(map[string]bool, len(known))
	for _, name := range known {
		knownSet[name] = true
	}
	out := make([]string, 0, len(file))
	for _, name := range file {
		if knownSet[name] {
			out = append(out, name)
		}
	}
	return out
}

// schemaFields returns every field of the live table schema.
func schemaFields(t *table.Table) []string {
	if t == nil {
		return nil
	}
	return t.FieldNames()
}

// valueOrDefault returns the row value for a field or the zero value when
// the row does not carry it.
func valueOrDefault(row Row, name string) (model.FeatureValue, bool) {
	value, ok := row.Fields[name]
	return value, ok
}
