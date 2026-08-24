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

// TestImportPreservesOnlineFieldsAddedMidWindow reproduces the regression
// where an offline import merged by schema field name overwrote fields the
// online side had just added during the half-hour import window. The import
// file only carries "age", so the overwrite window must be limited to "age";
// the online-added "city" and "level" fields must survive the merge untouched.
func TestImportPreservesOnlineFieldsAddedMidWindow(t *testing.T) {
	importer, featureStore := newTestImporter(t)
	// Online write happens first, carrying fields the import file will not
	// carry (simulating fields added online during the import window).
	onlineFields := map[string]model.FeatureValue{
		"age":   model.IntValue(25),
		"city":  model.StringValue("上海"),
		"level": model.IntValue(3),
	}
	if err := featureStore.Write("user_z", onlineFields, "v-online"); err != nil {
		t.Fatalf("online write: %v", err)
	}
	// The import file knows only about "age"; it must not clobber "city"
	// and "level", which are outside its overwrite window.
	rows := []Row{
		{Entity: "user_z", Fields: map[string]model.FeatureValue{"age": model.IntValue(40)}},
	}
	if _, err := importer.Import("batch-online", "user_profile", rows, "v-import"); err != nil {
		t.Fatalf("import: %v", err)
	}
	snapshot, err := featureStore.Read("user_z")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := snapshot.Fields["age"].Int; got != 40 {
		t.Fatalf("age = %d, want 40 (imported field must be overwritten)", got)
	}
	if got := snapshot.Fields["city"].Str; got != "上海" {
		t.Fatalf("city = %q, want %q (online-added field must be preserved)", got, "上海")
	}
	if got := snapshot.Fields["level"].Int; got != 3 {
		t.Fatalf("level = %d, want 3 (online-added field must be preserved)", got)
	}
}

// TestImportRemovesWindowedFieldsAbsentFromFile confirms that a field the
// import file DID carry on one entity but dropped on another is removed when
// absent, i.e. the window remains authoritative for fields the file knows
// about (no stale residue from the previous snapshot).
func TestImportRemovesWindowedFieldsAbsentFromFile(t *testing.T) {
	importer, featureStore := newTestImporter(t)
	if err := featureStore.Write("user_y", map[string]model.FeatureValue{
		"age":  model.IntValue(20),
		"city": model.StringValue("北京"),
	}, "v1"); err != nil {
		t.Fatalf("write: %v", err)
	}
	// The file still declares "age" elsewhere in the batch, so "age" is part
	// of the overwrite window; on user_y the file drops it, so it must be
	// removed to honor the imported snapshot.
	rows := []Row{
		{Entity: "user_y", Fields: map[string]model.FeatureValue{"city": model.StringValue("深圳")}},
		{Entity: "user_w", Fields: map[string]model.FeatureValue{"age": model.IntValue(99)}},
	}
	if _, err := importer.Import("batch-absent", "user_profile", rows, "v2"); err != nil {
		t.Fatalf("import: %v", err)
	}
	snapshot, _ := featureStore.Read("user_y")
	if got := snapshot.Fields["city"].Str; got != "深圳" {
		t.Fatalf("city = %q, want %q", got, "深圳")
	}
	if _, present := snapshot.Fields["age"]; present {
		t.Fatalf("age must be removed: windowed field absent from file row is stale")
	}
}
