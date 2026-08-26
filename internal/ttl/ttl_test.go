package ttl

import (
	"testing"
	"time"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func newTestScanner(t *testing.T, ttl time.Duration) (*Scanner, *store.Store) {
	t.Helper()
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
	return NewScanner(featureStore, ttl), featureStore
}

func TestScanRemovesOldEntries(t *testing.T) {
	scanner, featureStore := newTestScanner(t, 60*time.Second)
	now := time.Now().Truncate(time.Second)
	if err := featureStore.WriteAt("user_1", map[string]model.FeatureValue{"age": model.IntValue(1)}, "v1", now.Add(-120*time.Second)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := featureStore.WriteAt("user_2", map[string]model.FeatureValue{"age": model.IntValue(2)}, "v1", now.Add(-10*time.Second)); err != nil {
		t.Fatalf("write: %v", err)
	}
	expired := scanner.Scan(now)
	if expired != 1 {
		t.Fatalf("expired = %d, want 1", expired)
	}
	if featureStore.EntryFor("user_1") != nil {
		t.Fatal("old entity must be removed")
	}
	if featureStore.EntryFor("user_2") == nil {
		t.Fatal("fresh entity must survive")
	}
}
