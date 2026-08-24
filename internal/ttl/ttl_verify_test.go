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

func TestTTLExpiryBoundaryKeepsFreshWrites(t *testing.T) {
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

	now := time.Now().Truncate(time.Second)
	key := model.EntityKey("user_boundary")
	if err := featureStore.WriteAt(key, map[string]model.FeatureValue{
		"age": model.IntValue(20),
	}, "v1", now.Add(-60*time.Second)); err != nil {
		t.Fatalf("write at boundary: %v", err)
	}

	scanner := NewScanner(featureStore, 60*time.Second)
	scanner.Scan(now)

	if featureStore.EntryFor(key) == nil {
		t.Fatal("a feature written exactly on the TTL boundary was expired")
	}
}
