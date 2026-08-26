package ingest

import "fmt"

// Rollback restores every entity touched by the batch, including the segment
// that was still in flight when the batch failed, to its pre-import state.
func (im *Importer) Rollback(batchID string) error {
	batch, ok := im.batches[batchID]
	if !ok {
		return fmt.Errorf("ingest: unknown batch %q", batchID)
	}
	ops := im.store.Journal().OpsFor(batchID)
	applied := map[string]bool{}
	for _, segment := range batch.Segments {
		if !segment.Done {
			continue
		}
		for _, op := range ops {
			if op.Segment != segment.Index || applied[string(op.Key)] {
				continue
			}
			applied[string(op.Key)] = true
			if op.Before == nil {
				if err := im.store.Delete(op.Key); err != nil {
					return err
				}
				continue
			}
			if err := im.store.WriteAt(op.Key, op.Before.Fields, op.Before.VersionID, op.Before.WrittenAt); err != nil {
				return err
			}
		}
	}
	batch.Failed = true
	im.store.Journal().Clear(batchID)
	return nil
}
