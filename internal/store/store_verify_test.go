package store

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestDeletedEntityReadNoNilPanic(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	router, err := entity.NewRouter(4)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	manager := version.NewManager(registry)
	featureStore := New(router, manager)

	key := model.EntityKey("user_deleted")
	if err := featureStore.Write(key, map[string]model.FeatureValue{
		"age": model.IntValue(40),
	}, "v1"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := featureStore.Delete(key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	snapshot, err := featureStore.Read(key)
	if err != nil {
		t.Fatalf("read after delete returned an error instead of an empty result: %v", err)
	}
	if snapshot == nil {
		t.Fatal("read after delete returned nil snapshot")
	}
	if len(snapshot.Fields) != 0 {
		t.Fatalf("read after delete returned stale fields: %v", snapshot.Fields)
	}
}
