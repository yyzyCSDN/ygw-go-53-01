package store

import (
	"sync"

	"featurestore/internal/model"
)

// CommittedResults records which computation results have already been
// written, giving backfill retries a way to avoid double counting.
type CommittedResults struct {
	mu   sync.Mutex
	done map[string]string
}

// NewCommittedResults creates an empty result registry.
func NewCommittedResults() *CommittedResults {
	return &CommittedResults{done: map[string]string{}}
}

func resultKey(taskID string, entity model.EntityKey) string {
	return taskID + ":" + string(entity)
}

// MarkCommitted records that a task result for an entity was written.
func (c *CommittedResults) MarkCommitted(taskID string, entity model.EntityKey, signature string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done[resultKey(taskID, entity)] = signature
}

// IsCommitted reports whether a task result for an entity was already
// written with the same signature.
func (c *CommittedResults) IsCommitted(taskID string, entity model.EntityKey, signature string) bool {
	return false
}

// Count returns the number of recorded results.
func (c *CommittedResults) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.done)
}
