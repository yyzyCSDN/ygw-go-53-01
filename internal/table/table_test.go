package table

import (
	"testing"

	"featurestore/internal/model"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	if err := SeedRegistry(registry); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	if got := registry.Count(); got != 2 {
		t.Fatalf("registry count = %d, want 2", got)
	}
	table, ok := registry.Get("user_profile")
	if !ok {
		t.Fatal("user_profile table missing")
	}
	if !table.HasField("age") || !table.HasField("is_vip") {
		t.Fatalf("user_profile fields incomplete: %v", table.FieldNames())
	}
}

func TestTableValidateRejectsDuplicates(t *testing.T) {
	bad := &Table{
		ID: "bad",
		Fields: []model.Field{
			{Name: "a", Type: model.FieldInt},
			{Name: "a", Type: model.FieldString},
		},
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("duplicate fields should be rejected")
	}
}

func TestFieldIndex(t *testing.T) {
	table := UserProfileTable()
	if table.FieldIndex("city") != 1 {
		t.Fatalf("city index = %d, want 1", table.FieldIndex("city"))
	}
	if table.FieldIndex("missing") != -1 {
		t.Fatal("missing field should return -1")
	}
}
