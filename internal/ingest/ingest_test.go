package ingest

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func newTestImporter(t *testing.T) (*Importer, *store.Store) {
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
	return NewImporter(featureStore, registry, 2), featureStore
}

func TestImportWritesRows(t *testing.T) {
	importer, featureStore := newTestImporter(t)
	rows := []Row{
		{Entity: "user_a", Fields: map[string]model.FeatureValue{"age": model.IntValue(20)}},
		{Entity: "user_b", Fields: map[string]model.FeatureValue{"age": model.IntValue(30)}},
		{Entity: "user_c", Fields: map[string]model.FeatureValue{"age": model.IntValue(40)}},
	}
	batch, err := importer.Import("batch-1", "user_profile", rows, "v1")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(batch.Segments) != 2 {
		t.Fatalf("segments = %d, want 2 (window size 2)", len(batch.Segments))
	}
	for _, key := range []model.EntityKey{"user_a", "user_b", "user_c"} {
		snapshot, err := featureStore.Read(key)
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		if !snapshot.Fields["age"].Set {
			t.Fatalf("entity %s has no imported age field", key)
		}
	}
}

func TestImportEmptyFileRejected(t *testing.T) {
	importer, _ := newTestImporter(t)
	if _, err := importer.Import("batch-empty", "user_profile", nil, "v1"); err == nil {
		t.Fatal("empty import must be rejected")
	}
}

func TestRollbackAfterSuccessfulImport(t *testing.T) {
	importer, featureStore := newTestImporter(t)
	rows := []Row{
		{Entity: "user_x", Fields: map[string]model.FeatureValue{"age": model.IntValue(50)}},
	}
	if _, err := importer.Import("batch-ok", "user_profile", rows, "v1"); err != nil {
		t.Fatalf("import: %v", err)
	}
	if err := importer.Rollback("batch-ok"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if entry := featureStore.EntryFor("user_x"); entry != nil {
		t.Fatal("rolled-back entity must be absent")
	}
}
