package ingest

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestImportMergeKeepsIncrementalFields(t *testing.T) {
	registry := table.NewRegistry()
	profile := &table.Table{
		ID:   "ft",
		Name: "特征表",
		Fields: []model.Field{
			{Name: "age", Type: model.FieldInt},
			{Name: "city", Type: model.FieldString},
			{Name: "level", Type: model.FieldInt},
			{Name: "online_a", Type: model.FieldString},
			{Name: "online_b", Type: model.FieldString},
		},
	}
	if err := registry.Register(profile); err != nil {
		t.Fatalf("register table: %v", err)
	}
	router, err := entity.NewRouter(4)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	manager := version.NewManager(registry)
	featureStore := store.New(router, manager)
	importer := NewImporter(featureStore, registry, 2)

	key := model.EntityKey("user_incr")
	// Fields added online while the offline import is in flight.
	if err := featureStore.Write(key, map[string]model.FeatureValue{
		"online_a": model.StringValue("x"),
		"online_b": model.StringValue("y"),
	}, "v-online"); err != nil {
		t.Fatalf("online write: %v", err)
	}

	rows := []Row{
		{Entity: key, Fields: map[string]model.FeatureValue{
			"age":   model.IntValue(30),
			"city":  model.StringValue("杭州"),
			"level": model.IntValue(2),
		}},
	}
	if _, err := importer.Import("batch-incr", "ft", rows, "v-file"); err != nil {
		t.Fatalf("import: %v", err)
	}

	snapshot, err := featureStore.Read(key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !snapshot.Fields["online_a"].Set || snapshot.Fields["online_a"].Str != "x" {
		t.Fatalf("online-added field online_a was clobbered by the import: %v", snapshot.Fields["online_a"])
	}
	if !snapshot.Fields["online_b"].Set || snapshot.Fields["online_b"].Str != "y" {
		t.Fatalf("online-added field online_b was clobbered by the import: %v", snapshot.Fields["online_b"])
	}
	if snapshot.Fields["age"].Int != 30 {
		t.Fatalf("imported field age missing: %v", snapshot.Fields["age"])
	}
}
