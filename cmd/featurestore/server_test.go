package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
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

func newTestServer(t *testing.T) *Server {
	t.Helper()
	tables := table.NewRegistry()
	if err := table.SeedRegistry(tables); err != nil {
		t.Fatalf("seed: %v", err)
	}
	router, err := entity.NewRouter(16)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	versions := version.NewManager(tables)
	featureStore := store.New(router, versions)
	_, file, _, _ := runtime.Caller(0)
	webDir := filepath.Join(filepath.Dir(file), "../../web")
	return &Server{
		tables:    tables,
		versions:  versions,
		router:    router,
		store:     featureStore,
		importer:  ingest.NewImporter(featureStore, tables, 32),
		backfill:  compute.NewBackfill(featureStore),
		checker:   sync.NewChecker(featureStore, versions, 10*time.Minute),
		scanner:   ttl.NewScanner(featureStore, time.Hour),
		startedAt: time.Now(),
		webDir:    webDir,
	}
}

func TestHealthEndpoint(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("health body = %v", body)
	}
}

func TestBrowsePageServed(t *testing.T) {
	server := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/web/browse.html", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("browse status = %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("FeatureStore")) {
		t.Fatal("browse page missing expected title")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	server := newTestServer(t)
	payload := []byte(`{"key":"user_7","fields":{"age":28,"city":"深圳"}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/write", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("write status = %d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/read?key=user_7", nil)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("read status = %d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode read: %v", err)
	}
	fields, _ := body["fields"].(map[string]any)
	if fields["age"] != "28" {
		t.Fatalf("age = %v", fields["age"])
	}
}
