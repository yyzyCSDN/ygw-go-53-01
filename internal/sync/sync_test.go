package sync

import (
	"testing"
	"time"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestPartitionCoversRange(t *testing.T) {
	begin := time.Unix(0, 0)
	until := begin.Add(100 * time.Minute)
	windows := Partition(begin, until, 30*time.Minute)
	if len(windows) != 4 {
		t.Fatalf("windows = %d, want 4", len(windows))
	}
	if !windows[len(windows)-1].End.Equal(until) {
		t.Fatalf("last window end = %v, want %v", windows[len(windows)-1].End, until)
	}
}

func TestCheckNoDiffsWhenConsistent(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router, err := entity.NewRouter(4)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	manager := version.NewManager(registry)
	featureStore := store.New(router, manager)
	checker := NewChecker(featureStore, manager, 30*time.Minute)

	begin := time.Unix(0, 0)
	until := begin.Add(60 * time.Minute)
	diffs, err := checker.Check(begin, until, func(key model.EntityKey) *model.Snapshot {
		return model.EmptySnapshot(key, "offline")
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("diffs = %d, want 0", len(diffs))
	}
}
