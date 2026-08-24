package ingest

import "fmt"

// Rollback restores every entity touched by the batch, including the segment
// that was still in flight when the batch failed, to its pre-import state.
func (im *Importer) Rollback(batchID string) error {
	batch, ok := im.batches[batchID]
	if !ok {
		return fmt.Errorf("ingest: unknown batch %q", batchID)
	}
	batch.Failed = true
	return im.store.RollbackBatch(batchID)
}
