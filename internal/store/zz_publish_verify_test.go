package store

import (
	"testing"

	"featurestore/internal/model"
)

// TestPublishSwitchesActiveAndFlushesCache guards the version-publish path
// against a regression where publishing a new version updated only the
// version record but left the active pointer pointing at the old version and
// let the online read cache keep serving stale snapshots. After a publish the
// active version must switch immediately and cached reads of the prior
// version must be dropped so online reads never mix old and new values.
func TestPublishSwitchesActiveAndFlushesCache(t *testing.T) {
	store := newTestStore(t)
	manager := store.versions
	key := model.EntityKey("user_1")

	// v1 only knows {age}; v2 adds {score}.
	v1, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	v1.Fields = []string{"age"}
	if _, err := manager.Publish(v1.ID); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	// Write the entity carrying both fields and read under v1; score must be
	// hidden because v1's layout does not include it. This also warms the
	// read cache keyed by the active version.
	if err := store.Write(key, map[string]model.FeatureValue{
		"age":   model.IntValue(30),
		"score": model.FloatValue(9.5),
	}, v1.ID); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	if snap, _ := store.Read(key); snap.Fields["score"].Set {
		t.Fatalf("v1 layout must hide score, got %v", snap.Fields)
	}

	// Publish v2 with the expanded layout {age, score}.
	v2, err := manager.Create("user_profile")
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	v2.Fields = []string{"age", "score"}
	if _, err := manager.Publish(v2.ID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	// The active pointer must switch immediately, without a restart.
	if active := manager.ResolveActive(); active == nil || active.ID != v2.ID {
		t.Fatalf("active version = %v, want %s (publish did not switch active pointer)", active, v2.ID)
	}

	// A read right after publish must use v2's layout, i.e. surface score,
	// proving the cache no longer serves the v1-filtered snapshot.
	snap, err := store.Read(key)
	if err != nil {
		t.Fatalf("read after publish: %v", err)
	}
	if score, ok := snap.Fields["score"]; !ok || score.Float != 9.5 {
		t.Fatalf("read after publish = %v, want score=9.5 (cache served stale v1 snapshot)", snap.Fields)
	}

	// The prior active version must be superseded, not still published.
	if prior, _ := manager.Get(v1.ID); prior.State != model.VersionSuperseded {
		t.Fatalf("prior version state = %s, want superseded", prior.State)
	}
}
