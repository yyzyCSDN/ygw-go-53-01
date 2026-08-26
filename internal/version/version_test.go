package version

import (
	"testing"

	"featurestore/internal/model"
	"featurestore/internal/table"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewManager(registry)
}

func TestCreateAndList(t *testing.T) {
	manager := newTestManager(t)
	first, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("version ids must be unique")
	}
	if got := len(manager.List()); got != 2 {
		t.Fatalf("list len = %d, want 2", got)
	}
}

func TestTransitionRules(t *testing.T) {
	if !CanTransition(model.VersionDraft, model.VersionPublished) {
		t.Fatal("draft -> published must be allowed")
	}
	if CanTransition(model.VersionPublished, model.VersionDraft) {
		t.Fatal("published -> draft must be rejected")
	}
	if CanTransition(model.VersionRolledBack, model.VersionPublished) {
		t.Fatal("rolled-back -> published must be rejected")
	}
}

func TestPublishRejectsNonDraft(t *testing.T) {
	manager := newTestManager(t)
	first, err := manager.Create("item_profile")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := manager.Publish(first.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := manager.Publish(first.ID); err == nil {
		t.Fatal("publishing an already published version must fail")
	}
}

func TestRollbackStateChange(t *testing.T) {
	manager := newTestManager(t)
	first, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	first, err = manager.Publish(first.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	second, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	second, err = manager.Publish(second.ID)
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}
	parent, err := manager.Rollback(second.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if parent.ID != first.ID {
		t.Fatalf("rollback parent = %s, want %s", parent.ID, first.ID)
	}
	if rolled, _ := manager.Get(second.ID); rolled.State != model.VersionRolledBack {
		t.Fatalf("rolled-back version state = %s", rolled.State)
	}
}
