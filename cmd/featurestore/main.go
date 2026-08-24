// Command featurestore starts the feature store demo service with an HTTP
// API and a browser-based feature browsing page.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"featurestore/internal/compute"
	"featurestore/internal/entity"
	"featurestore/internal/ingest"
	"featurestore/internal/store"
	"featurestore/internal/sync"
	"featurestore/internal/table"
	"featurestore/internal/ttl"
	"featurestore/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	shards := flag.Int("shards", 16, "initial shard count")
	ttlValue := flag.Duration("ttl", 24*time.Hour, "default feature TTL")
	flag.Parse()

	tables := table.NewRegistry()
	if err := table.SeedRegistry(tables); err != nil {
		log.Fatalf("seed tables: %v", err)
	}
	router, err := entity.NewRouter(*shards)
	if err != nil {
		log.Fatalf("router: %v", err)
	}
	versions := version.NewManager(tables)
	featureStore := store.New(router, versions)
	featureStore.SetClock(time.Now)
	importer := ingest.NewImporter(featureStore, tables, 32)
	backfill := compute.NewBackfill(featureStore)
	checker := sync.NewChecker(featureStore, versions, 10*time.Minute)
	scanner := ttl.NewScanner(featureStore, *ttlValue)
	initial, err := versions.Create("user_profile")
	if err != nil {
		log.Fatalf("create initial version: %v", err)
	}
	if _, err := versions.Publish(initial.ID); err != nil {
		log.Fatalf("publish initial version: %v", err)
	}
	if _, err := versions.Create("item_profile"); err != nil {
		log.Fatalf("create item version: %v", err)
	}

	server := &Server{
		tables:    tables,
		versions:  versions,
		router:    router,
		store:     featureStore,
		importer:  importer,
		backfill:  backfill,
		checker:   checker,
		scanner:   scanner,
		startedAt: time.Now(),
	}
	handler := server.Handler()

	log.Printf("featurestore listening on %s (shards=%d ttl=%s)", *addr, *shards, *ttlValue)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
