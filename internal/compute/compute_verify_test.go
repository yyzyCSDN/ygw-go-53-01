package compute

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestBackfillChecksVersionBeforeWrite(t *testing.T) {
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
	backfill := NewBackfill(featureStore)

	key := model.EntityKey("user_backfill")
	// Online update lands first with a newer version.
	if err := featureStore.Write(key, map[string]model.FeatureValue{
		"score": model.IntValue(100),
	}, "t-v3"); err != nil {
		t.Fatalf("online write: %v", err)
	}
	oldVersion := model.Version{ID: "t-v1", TableID: "user_profile"}
	if _, err := backfill.Run("task-v", oldVersion, []Result{
		{Entity: key, Fields: map[string]model.FeatureValue{"score": model.IntValue(5)}},
	}); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	snapshot, err := featureStore.Read(key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if snapshot.Fields["score"].Int != 100 {
		t.Fatalf("backfill overwrote the newer online value with %d", snapshot.Fields["score"].Int)
	}
}
