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

func TestSyncWindowNoGap(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	router, err := entity.NewRouter(4)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	manager := version.NewManager(registry)
	featureStore := store.New(router, manager)
	checker := NewChecker(featureStore, manager, 30*time.Minute)

	begin := time.Unix(0, 0)
	until := begin.Add(100 * time.Minute)
	key := model.EntityKey("user_window")
	// The only divergence sits inside the final partial window [90,100).
	if err := featureStore.WriteAt(key, map[string]model.FeatureValue{
		"age": model.IntValue(33),
	}, "v1", begin.Add(95*time.Minute)); err != nil {
		t.Fatalf("write in tail window: %v", err)
	}

	offlineView := func(k model.EntityKey) *model.Snapshot {
		return model.EmptySnapshot(k, "offline")
	}
	diffs, err := checker.Check(begin, until, offlineView)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(diffs) == 0 {
		t.Fatal("the tail window was skipped: divergence in the switch window was not detected")
	}
}
