package version

import (
	"testing"

	"featurestore/internal/table"
)

type recordingHook struct {
	calls []string
}

func (h *recordingHook) InvalidateVersion(versionID string) {
	h.calls = append(h.calls, versionID)
}

func (h *recordingHook) InvalidateAll() {
	h.calls = append(h.calls, "*")
}

func TestRollbackInvalidatesOnlineCache(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	manager := NewManager(registry)
	hook := &recordingHook{}
	manager.SetCacheHook(hook)

	first, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := manager.Publish(first.ID); err != nil {
		t.Fatalf("publish first: %v", err)
	}
	second, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := manager.Publish(second.ID); err != nil {
		t.Fatalf("publish second: %v", err)
	}

	if _, err := manager.Rollback(second.ID); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if len(hook.calls) == 0 {
		t.Fatal("rollback did not invalidate the online cache")
	}
}
