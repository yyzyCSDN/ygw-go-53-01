package compute

import (
	"testing"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

func TestComputeRetryChecksCommittedResult(t *testing.T) {
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

	version := model.Version{ID: "t-v1", TableID: "user_profile"}
	rows := []Result{
		{Entity: "user_cnt", Fields: map[string]model.FeatureValue{"cnt": model.IntValue(5)}},
	}
	// First attempt commits the result but the caller believes it failed and
	// retries the same task.
	if _, err := backfill.Retry("task-cnt", version, rows); err != nil {
		t.Fatalf("first retry attempt: %v", err)
	}
	if _, err := backfill.Retry("task-cnt", version, rows); err != nil {
		t.Fatalf("second retry attempt: %v", err)
	}

	snapshot, err := featureStore.Read("user_cnt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if snapshot.Fields["cnt"].Int != 5 {
		t.Fatalf("retry re-applied the committed result: cnt = %d, want 5", snapshot.Fields["cnt"].Int)
	}
}
