package main

import (
	"encoding/json"
	"net/http"
	"time"

	"featurestore/internal/compute"
	"featurestore/internal/entity"
	"featurestore/internal/ingest"
	"featurestore/internal/model"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	tables := make([]string, 0, s.tables.Count())
	for _, definition := range s.tables.List() {
		tables = append(tables, definition.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"tables":    tables,
		"shards":    s.router.ShardCount(),
		"startedAt": s.startedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	key := model.EntityKey(r.URL.Query().Get("key"))
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing key"})
		return
	}
	parsed, err := entity.ParseEntity(string(key))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	key = parsed
	snapshot, err := s.store.Read(key)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entity":    snapshot.Entity,
		"versionID": snapshot.VersionID,
		"fields":    fieldsAsStrings(snapshot),
	})
}

func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key       string                       `json:"key"`
		VersionID string                       `json:"versionID"`
		Fields    map[string]json.RawMessage   `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing key"})
		return
	}
	parsed, err := entity.ParseEntity(req.Key)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	fields, err := decodeFields(req.Fields)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.Write(parsed, fields, req.VersionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"written": true})
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VersionID string `json:"versionID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	version, err := s.versions.Publish(req.VersionID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": version.ID})
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VersionID string `json:"versionID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	parent, err := s.versions.Rollback(req.VersionID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": parent.ID})
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BatchID   string                       `json:"batchID"`
		TableID   string                       `json:"tableID"`
		VersionID string                       `json:"versionID"`
		Rows      []importRow                  `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rows := make([]ingest.Row, 0, len(req.Rows))
	for _, row := range req.Rows {
		fields, err := decodeFields(row.Fields)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rows = append(rows, ingest.Row{Entity: model.EntityKey(row.Key), Fields: fields})
	}
	batch, err := s.importer.Import(req.BatchID, req.TableID, rows, req.VersionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "batch": batch})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batchID": batch.ID, "segments": len(batch.Segments)})
}

func (s *Server) handleBackfill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID    string                       `json:"taskID"`
		VersionID string                       `json:"versionID"`
		Retry     bool                         `json:"retry"`
		Rows      []importRow                  `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	version, ok := s.versions.Get(req.VersionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "version not found"})
		return
	}
	rows := make([]compute.Result, 0, len(req.Rows))
	for _, row := range req.Rows {
		fields, err := decodeFields(row.Fields)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rows = append(rows, compute.Result{Entity: model.EntityKey(row.Key), Fields: fields})
	}
	var written int
	var err error
	if req.Retry {
		written, err = s.backfill.Retry(req.TaskID, *version, rows)
	} else {
		written, err = s.backfill.Run(req.TaskID, *version, rows)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": written})
}

func (s *Server) handleExpand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Shards int `json:"shards"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	expansion, err := s.router.Expand(req.Shards)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.ExpandShards(req.Shards); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	states := make([]int, 0, s.router.ShardCount())
	for shard := 0; shard < s.router.ShardCount(); shard++ {
		states = append(states, s.router.ShardState(shard))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"previous": expansion.Previous,
		"current":  expansion.Current,
		"added":    expansion.Added,
		"shards":   states,
	})
}

func (s *Server) handleSyncCheck(w http.ResponseWriter, r *http.Request) {
	begin, _ := time.Parse(time.RFC3339, r.URL.Query().Get("begin"))
	until, _ := time.Parse(time.RFC3339, r.URL.Query().Get("until"))
	if until.IsZero() {
		until = time.Now()
	}
	if begin.IsZero() {
		begin = until.Add(-time.Hour)
	}
	diffs, err := s.checker.Check(begin, until, s.offlineView)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diffs": diffs, "count": len(diffs)})
}

func (s *Server) handleTTLScan(w http.ResponseWriter, r *http.Request) {
	expired := s.scanner.Scan(time.Now())
	writeJSON(w, http.StatusOK, map[string]int{"expired": expired})
}

func (s *Server) handleVersions(w http.ResponseWriter, _ *http.Request) {
	versions := s.versions.List()
	out := make([]map[string]any, 0, len(versions))
	for _, v := range versions {
		out = append(out, map[string]any{
			"id":    v.ID,
			"table": v.TableID,
			"state": v.State,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": out})
}

func (s *Server) offlineView(_ model.EntityKey) *model.Snapshot {
	return nil
}

type importRow struct {
	Key    string                     `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
}

func fieldsAsStrings(snapshot *model.Snapshot) map[string]string {
	out := map[string]string{}
	for name, value := range snapshot.Fields {
		out[name] = value.String()
	}
	return out
}

func decodeFields(raw map[string]json.RawMessage) (map[string]model.FeatureValue, error) {
	fields := map[string]model.FeatureValue{}
	for name, data := range raw {
		var text string
		if err := json.Unmarshal(data, &text); err == nil {
			fields[name] = model.StringValue(text)
			continue
		}
		var number float64
		if err := json.Unmarshal(data, &number); err == nil {
			if number == float64(int64(number)) {
				fields[name] = model.IntValue(int64(number))
			} else {
				fields[name] = model.FloatValue(number)
			}
			continue
		}
		var flag bool
		if err := json.Unmarshal(data, &flag); err == nil {
			fields[name] = model.BoolValue(flag)
			continue
		}
		return nil, &decodeError{name: name}
	}
	return fields, nil
}

type decodeError struct{ name string }

func (e *decodeError) Error() string {
	return "cannot decode field " + jsonQuote(e.name)
}

func jsonQuote(name string) string {
	data, _ := json.Marshal(name)
	return string(data)
}
