package sync

import (
	"testing"
	"time"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
	"featurestore/internal/version"
)

// TestCheckDetectsTailSwitchWindowDiff guards against a regression where the
// consistency check dropped the trailing partial step window. A version
// switch lands inside that tail window and the online value diverges from the
// offline view; the check must report the diff instead of silently passing.
func TestCheckDetectsTailSwitchWindowDiff(t *testing.T) {
	registry := table.NewRegistry()
	if err := table.SeedRegistry(registry); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router, err := entity.NewRouter(4)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	manager := version.NewManager(registry)
	// Deterministic clock so the publish time lands inside the checked range.
	manager.SetClock(func() time.Time { return time.Unix(1800, 0) })
	featureStore := store.New(router, manager)
	checker := NewChecker(featureStore, manager, time.Hour)

	key := model.EntityKey("user_sw")
	v1, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := manager.Publish(v1.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// Write inside the tail window [60min, 90min) of a 0..90min range.
	if err := featureStore.WriteAt(key, map[string]model.FeatureValue{
		"age": model.IntValue(42),
	}, v1.ID, time.Unix(4000, 0)); err != nil {
		t.Fatalf("write: %v", err)
	}

	begin := time.Unix(0, 0)
	until := begin.Add(90 * time.Minute)
	diffs, err := checker.Check(begin, until, func(key model.EntityKey) *model.Snapshot {
		return &model.Snapshot{
			Entity: key,
			Fields: map[string]model.FeatureValue{"age": model.IntValue(7)},
		}
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(diffs) == 0 {
		t.Fatal("diff in tail switch window was not reported; window boundary is not contiguous")
	}
}
