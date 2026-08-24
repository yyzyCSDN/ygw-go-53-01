// Package ingest implements the offline batch importer that merges feature
// files into the store without clobbering fields written online.
package ingest

import (
	"fmt"
	"sort"
	"time"

	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/table"
)

// Row is one entity line of an import file.
type Row struct {
	Entity model.EntityKey
	Fields map[string]model.FeatureValue
}

// Segment is one chunk of an import batch.
type Segment struct {
	Index int
	Done  bool
	Rows  int
}

// Batch is the mutable state of one import operation.
type Batch struct {
	ID       string
	VersionID string
	Segments []Segment
	Started  time.Time
	Failed   bool
}

// Importer merges import files into the store in segments.
type Importer struct {
	store      *store.Store
	tables     *table.Registry
	windowSize int
	batches    map[string]*Batch
}

// NewImporter creates an importer with the given row window size.
func NewImporter(s *store.Store, tables *table.Registry, windowSize int) *Importer {
	if windowSize <= 0 {
		windowSize = 64
	}
	return &Importer{
		store:      s,
		tables:     tables,
		windowSize: windowSize,
		batches:    map[string]*Batch{},
	}
}

// fileFields returns the sorted union of field names present in the import
// file. Only these fields may be overwritten by the merge.
func fileFields(rows []Row) []string {
	set := map[string]bool{}
	for _, row := range rows {
		for name := range row.Fields {
			set[name] = true
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Import merges a file into the store in row windows and records the batch
// for later rollback. An error mid-batch leaves the batch failed but does
// not roll back automatically; callers decide.
func (im *Importer) Import(batchID, tableID string, rows []Row, versionID string) (*Batch, error) {
	if batchID == "" {
		return nil, fmt.Errorf("ingest: batch id must not be empty")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ingest: empty import file")
	}
	overwrite := fieldWindow(im.tables.MustGet(tableID), rows)
	batch := &Batch{ID: batchID, VersionID: versionID, Started: time.Now()}
	im.batches[batchID] = batch
	for start := 0; start < len(rows); start += im.windowSize {
		end := start + im.windowSize
		if end > len(rows) {
			end = len(rows)
		}
		segment := Segment{Index: len(batch.Segments)}
		batch.Segments = append(batch.Segments, segment)
		for _, row := range rows[start:end] {
			if err := im.writeRow(row, overwrite, versionID, batchID, segment.Index); err != nil {
				batch.Failed = true
				return batch, err
			}
			segment.Rows++
			batch.Segments[len(batch.Segments)-1] = segment
		}
		batch.Segments[len(batch.Segments)-1].Done = true
	}
	return batch, nil
}

// writeRow merges one import row into the store and journals the previous
// entry so the whole batch can be reverted later.
func (im *Importer) writeRow(row Row, overwrite []string, versionID, batchID string, segment int) error {
	before := im.store.EntryFor(row.Entity)
	im.store.Journal().Append(store.JournalOp{
		BatchID: batchID,
		Segment: segment,
		Key:     row.Entity,
		Before:  before,
	})
	patch := make(map[string]model.FeatureValue, len(overwrite))
	for _, name := range overwrite {
		if value, ok := valueOrDefault(row, name); ok {
			patch[name] = value
		}
	}
	return im.store.MergeWrite(row.Entity, patch, overwrite, versionID)
}

// Batch returns the recorded batch state.
func (im *Importer) Batch(batchID string) (*Batch, bool) {
	b, ok := im.batches[batchID]
	return b, ok
}

// Clear forgets a batch after successful handling.
func (im *Importer) Clear(batchID string) {
	delete(im.batches, batchID)
	im.store.Journal().Clear(batchID)
}
