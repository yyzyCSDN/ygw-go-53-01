package version

import (
	"testing"

	"featurestore/internal/table"
)

func TestReadUsesPublishedVersion(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	manager := NewManager(registry)

	first, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create first version: %v", err)
	}
	if _, err := manager.Publish(first.ID); err != nil {
		t.Fatalf("publish first version: %v", err)
	}
	second, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create second version: %v", err)
	}
	if _, err := manager.Publish(second.ID); err != nil {
		t.Fatalf("publish second version: %v", err)
	}

	active := manager.ResolveActive()
	if active == nil {
		t.Fatal("active version pointer is nil after publishing the second version")
	}
	if active.ID != second.ID {
		t.Fatalf("online read still resolves old version %q, want %q", active.ID, second.ID)
	}
}
