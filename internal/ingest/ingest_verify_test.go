package ingest

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestImportRollbackCoversAllSegments(t *testing.T) {
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
	importer := NewImporter(featureStore, registry, 1)

	rows := []Row{
		{Entity: "seg_1", Fields: map[string]model.FeatureValue{"age": model.IntValue(1)}},
		{Entity: "seg_2", Fields: map[string]model.FeatureValue{"age": model.IntValue(2)}},
	}
	if _, err := importer.Import("batch-rollback", "user_profile", rows, "v1"); err != nil {
		t.Fatalf("import: %v", err)
	}
	// Simulate a write from the segment that was still in flight when the
	// batch failed: it is journaled but its segment is never marked done.
	inflight := Row{Entity: "seg_3", Fields: map[string]model.FeatureValue{"age": model.IntValue(3)}}
	if err := importer.writeRow(inflight, []string{"age"}, "v1", "batch-rollback", 2); err != nil {
		t.Fatalf("in-flight write: %v", err)
	}

	if err := importer.Rollback("batch-rollback"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, key := range []model.EntityKey{"seg_1", "seg_2", "seg_3"} {
		if entry := featureStore.EntryFor(key); entry != nil {
			t.Fatalf("entity %s still holds imported state after rollback", key)
		}
	}
}
