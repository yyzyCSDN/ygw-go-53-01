// Package model defines the core domain types shared by every component of
// the feature store: entities, fields, snapshots and version markers.
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// EntityKey identifies one feature entity inside a feature table.
type EntityKey string

// FieldType describes the value kind of a feature field.
type FieldType string

const (
	FieldString FieldType = "string"
	FieldInt    FieldType = "int"
	FieldFloat  FieldType = "float"
	FieldBool   FieldType = "bool"
)

// Field is the schema definition of one named feature column.
type Field struct {
	Name string
	Type FieldType
}

// FeatureValue carries the typed value of a single feature field.
type FeatureValue struct {
	Type  FieldType
	Str   string
	Int   int64
	Float float64
	Bool  bool
	Set   bool
}

// String returns a canonical string form used by logs and the browse page.
func (v FeatureValue) String() string {
	switch v.Type {
	case FieldInt:
		return fmt.Sprintf("%d", v.Int)
	case FieldFloat:
		return fmt.Sprintf("%g", v.Float)
	case FieldBool:
		return fmt.Sprintf("%t", v.Bool)
	default:
		return v.Str
	}
}

// CloneValue deep-copies a feature value.
func CloneValue(v FeatureValue) FeatureValue {
	out := v
	out.Str = strings.Clone(v.Str)
	return out
}

// Snapshot is an immutable view of one entity at a specific version.
type Snapshot struct {
	Entity    EntityKey
	VersionID string
	Fields    map[string]FeatureValue
	WrittenAt time.Time
}

// Clone returns a deep copy of the snapshot.
func (s *Snapshot) Clone() *Snapshot {
	out := &Snapshot{
		Entity:    s.Entity,
		VersionID: s.VersionID,
		Fields:    make(map[string]FeatureValue, len(s.Fields)),
		WrittenAt: s.WrittenAt,
	}
	for name, value := range s.Fields {
		out.Fields[name] = CloneValue(value)
	}
	return out
}

// FieldNames returns the snapshot field names in sorted order.
func (s *Snapshot) FieldNames() []string {
	names := make([]string, 0, len(s.Fields))
	for name := range s.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EmptySnapshot returns a snapshot with no fields, used when an entity has
// never been written or was removed.
func EmptySnapshot(key EntityKey, versionID string) *Snapshot {
	return &Snapshot{
		Entity:    key,
		VersionID: versionID,
		Fields:    map[string]FeatureValue{},
	}
}

// StringValue builds a string-typed feature value.
func StringValue(v string) FeatureValue {
	return FeatureValue{Type: FieldString, Str: v, Set: true}
}

// IntValue builds an int-typed feature value.
func IntValue(v int64) FeatureValue {
	return FeatureValue{Type: FieldInt, Int: v, Set: true}
}

// FloatValue builds a float-typed feature value.
func FloatValue(v float64) FeatureValue {
	return FeatureValue{Type: FieldFloat, Float: v, Set: true}
}

// BoolValue builds a bool-typed feature value.
func BoolValue(v bool) FeatureValue {
	return FeatureValue{Type: FieldBool, Bool: v, Set: true}
}
