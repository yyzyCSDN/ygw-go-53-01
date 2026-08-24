package main

import (
	"net/http"
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

// Server bundles every component behind one HTTP handler.
type Server struct {
	tables    *table.Registry
	versions  *version.Manager
	router    *entity.Router
	store     *store.Store
	importer  *ingest.Importer
	backfill  *compute.Backfill
	checker   *sync.Checker
	scanner   *ttl.Scanner
	startedAt time.Time
	webDir    string
}

// Handler returns the HTTP mux with all API routes and the browse page.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/read", s.handleRead)
	mux.HandleFunc("/api/write", s.handleWrite)
	mux.HandleFunc("/api/publish", s.handlePublish)
	mux.HandleFunc("/api/rollback", s.handleRollback)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/api/backfill", s.handleBackfill)
	mux.HandleFunc("/api/expand", s.handleExpand)
	mux.HandleFunc("/api/sync-check", s.handleSyncCheck)
	mux.HandleFunc("/api/ttl-scan", s.handleTTLScan)
	mux.HandleFunc("/api/versions", s.handleVersions)
	webDir := s.webDir
	if webDir == "" {
		webDir = "web"
	}
	mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir(webDir))))
	return logRequests(mux)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
