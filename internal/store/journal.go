package store

import (
	"sync"

	"featurestore/internal/model"
)

// JournalOp records one write that must be reverted when an import batch is
// rolled back.
type JournalOp struct {
	BatchID  string
	Segment  int
	Key      model.EntityKey
	Before   *Entry
}

// Journal is the append-only write journal of the store.
type Journal struct {
	mu    sync.RWMutex
	ops   []JournalOp
	byKey map[model.EntityKey][]int
}

// NewJournal creates an empty journal.
func NewJournal() *Journal {
	return &Journal{byKey: map[model.EntityKey][]int{}}
}

// Append records an operation with the entry state that existed before the
// write.
func (j *Journal) Append(op JournalOp) {
	j.mu.Lock()
	defer j.mu.Unlock()
	index := len(j.ops)
	j.ops = append(j.ops, op)
	j.byKey[op.Key] = append(j.byKey[op.Key], index)
}

// OpsFor returns the journal operations of one batch in append order.
func (j *Journal) OpsFor(batchID string) []JournalOp {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]JournalOp, 0)
	for _, op := range j.ops {
		if op.BatchID == batchID {
			out = append(out, op)
		}
	}
	return out
}

// OpsForSegment returns the journal operations of one batch segment.
func (j *Journal) OpsForSegment(batchID string, segment int) []JournalOp {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]JournalOp, 0)
	for _, op := range j.ops {
		if op.BatchID == batchID && op.Segment == segment {
			out = append(out, op)
		}
	}
	return out
}

// LastBefore returns the most recent before-state recorded for a key.
func (j *Journal) LastBefore(key model.EntityKey) *Entry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	indexes := j.byKey[key]
	if len(indexes) == 0 {
		return nil
	}
	return j.ops[indexes[len(indexes)-1]].Before
}

// Clear removes all journal entries (used after a successful import).
func (j *Journal) Clear(batchID string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	kept := j.ops[:0]
	for _, op := range j.ops {
		if op.BatchID != batchID {
			kept = append(kept, op)
		}
	}
	j.ops = kept
	j.rebuildIndex()
}

func (j *Journal) rebuildIndex() {
	j.byKey = map[model.EntityKey][]int{}
	for i, op := range j.ops {
		j.byKey[op.Key] = append(j.byKey[op.Key], i)
	}
}
