package entity

import (
	"testing"

	"featurestore/internal/model"
)

func TestRouterInitialRouting(t *testing.T) {
	router, err := NewRouter(16)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	key := model.EntityKey("user_10001")
	shard := router.Route(key)
	if shard < 0 || shard >= 16 {
		t.Fatalf("shard out of range: %d", shard)
	}
	if router.Lookup(key) != shard {
		t.Fatal("lookup must agree with route")
	}
	if router.ShardCount() != 16 {
		t.Fatalf("shard count = %d", router.ShardCount())
	}
}

func TestHashKeyIsStable(t *testing.T) {
	first := HashKey("entity-a")
	second := HashKey("entity-a")
	if first != second {
		t.Fatal("hash must be stable")
	}
	if Slot("entity-a", 16) != Slot("entity-a", 16) {
		t.Fatal("slot must be stable")
	}
}

func TestExpandReportsDelta(t *testing.T) {
	router, err := NewRouter(16)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	expansion, err := router.Expand(64)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if expansion.Previous != 16 || expansion.Current != 64 || expansion.Added != 48 {
		t.Fatalf("expansion = %+v", expansion)
	}
}

func TestExpandRejectsShrink(t *testing.T) {
	router, err := NewRouter(16)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	if _, err := router.Expand(8); err == nil {
		t.Fatal("shrinking must be rejected")
	}
}

func TestParseEntityValidation(t *testing.T) {
	if _, err := ParseEntity("  "); err == nil {
		t.Fatal("blank entity key must be rejected")
	}
	key, err := ParseEntity(" user_ok ")
	if err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	if key != "user_ok" {
		t.Fatalf("parsed key = %q, want user_ok", key)
	}
}

func TestRouteEntity(t *testing.T) {
	router, err := NewRouter(8)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	shard, err := router.RouteEntity("user_route")
	if err != nil {
		t.Fatalf("route entity: %v", err)
	}
	if shard < 0 || shard >= 8 {
		t.Fatalf("shard out of range: %d", shard)
	}
	if _, err := router.RouteEntity(""); err == nil {
		t.Fatal("empty key must be rejected by RouteEntity")
	}
}
