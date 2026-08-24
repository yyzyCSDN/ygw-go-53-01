package store

import (
	"testing"
	"time"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func newTestStore(t *testing.T) *Store {
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
	return New(router, manager)
}

func TestWriteAndRead(t *testing.T) {
	store := newTestStore(t)
	key := model.EntityKey("user_1")
	fields := map[string]model.FeatureValue{"age": model.IntValue(30)}
	if err := store.Write(key, fields, "v1"); err != nil {
		t.Fatalf("write: %v", err)
	}
	snapshot, err := store.Read(key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if snapshot.Fields["age"].Int != 30 {
		t.Fatalf("age = %v", snapshot.Fields["age"])
	}
}

func TestMergeWriteUpdatesWindowFields(t *testing.T) {
	store := newTestStore(t)
	key := model.EntityKey("user_2")
	base := map[string]model.FeatureValue{
		"age":   model.IntValue(25),
		"city":  model.StringValue("上海"),
		"level": model.IntValue(3),
	}
	if err := store.Write(key, base, "v1"); err != nil {
		t.Fatalf("write: %v", err)
	}
	patch := map[string]model.FeatureValue{"city": model.StringValue("北京")}
	if err := store.MergeWrite(key, patch, []string{"city"}, "v2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	snapshot, _ := store.Read(key)
	if snapshot.Fields["city"].Str != "北京" {
		t.Fatalf("patched field not updated: %v", snapshot.Fields["city"])
	}
}

func TestDeleteRemovesEntity(t *testing.T) {
	store := newTestStore(t)
	key := model.EntityKey("user_3")
	if err := store.Write(key, map[string]model.FeatureValue{"age": model.IntValue(1)}, "v1"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := store.Delete(key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if entry := store.EntryFor(key); entry != nil {
		t.Fatal("deleted entity must be absent")
	}
}

func TestCompareAndWriteRejectsOlderVersion(t *testing.T) {
	store := newTestStore(t)
	key := model.EntityKey("user_5")
	if err := store.Write(key, map[string]model.FeatureValue{"score": model.IntValue(100)}, "t-v2"); err != nil {
		t.Fatalf("write: %v", err)
	}
	old := model.Version{ID: "t-v1"}
	written, err := store.CompareAndWrite(key, map[string]model.FeatureValue{"score": model.IntValue(5)}, old)
	if err != nil {
		t.Fatalf("compare write: %v", err)
	}
	if written {
		t.Fatal("older version must not overwrite a newer value")
	}
	snapshot, _ := store.Read(key)
	if snapshot.Fields["score"].Int != 100 {
		t.Fatalf("score overwritten: %v", snapshot.Fields["score"])
	}
}

func TestWrittenAtTracksWrite(t *testing.T) {
	store := newTestStore(t)
	key := model.EntityKey("user_6")
	at := time.Now().Truncate(time.Second)
	if err := store.WriteAt(key, map[string]model.FeatureValue{"age": model.IntValue(1)}, "v1", at); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, ok := store.WrittenAt(key)
	if !ok {
		t.Fatal("written at must be available")
	}
	if !got.Equal(at) {
		t.Fatalf("written at = %v, want %v", got, at)
	}
}
