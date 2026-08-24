package compute

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func newTestBackfill(t *testing.T) (*Backfill, *store.Store) {
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
	return NewBackfill(featureStore), featureStore
}

func TestBackfillWritesRows(t *testing.T) {
	backfill, featureStore := newTestBackfill(t)
	version := model.Version{ID: "t-v1", TableID: "user_profile"}
	rows := []Result{
		{Entity: "user_1", Fields: map[string]model.FeatureValue{"score": model.IntValue(10)}},
		{Entity: "user_2", Fields: map[string]model.FeatureValue{"score": model.IntValue(20)}},
	}
	written, err := backfill.Run("task-1", version, rows)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if written != 2 {
		t.Fatalf("written = %d, want 2", written)
	}
	task, ok := backfill.Task("task-1")
	if !ok || task.State != model.TaskDone || task.Processed != 2 {
		t.Fatalf("task state = %+v", task)
	}
	snapshot, _ := featureStore.Read("user_1")
	if snapshot.Fields["score"].Int != 10 {
		t.Fatalf("score = %v", snapshot.Fields["score"])
	}
}

func TestBuildPlanDeterministic(t *testing.T) {
	rows := []Result{
		{Entity: "b", Fields: map[string]model.FeatureValue{"age": model.IntValue(1)}},
		{Entity: "a", Fields: map[string]model.FeatureValue{"age": model.IntValue(2)}},
	}
	plan := BuildPlan(rows)
	if plan.Order[0] != "a" || plan.Order[1] != "b" {
		t.Fatalf("plan order = %v", plan.Order)
	}
}

func TestSignatureStable(t *testing.T) {
	rows := []Result{
		{Entity: "u", Fields: map[string]model.FeatureValue{"cnt": model.IntValue(7)}},
	}
	first := Signature("task-s", rows)
	second := Signature("task-s", rows)
	if first != second {
		t.Fatal("signature must be stable")
	}
}
