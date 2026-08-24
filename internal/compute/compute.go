// Package compute implements backfill and feature computation tasks with
// version guards and retry idempotency.
package compute

import (
	"fmt"
	"time"

	"featurestore/internal/model"
	"featurestore/internal/store"
)

// Result is one computed feature value for an entity.
type Result struct {
	Entity model.EntityKey
	Fields map[string]model.FeatureValue
}

// Backfill runs a backfill task: every row is written only when the incoming
// version is not older than the value already stored online.
type Backfill struct {
	store      *store.Store
	tasks      map[string]*model.Task
}

// NewBackfill creates a backfill runner.
func NewBackfill(s *store.Store) *Backfill {
	return &Backfill{store: s, tasks: map[string]*model.Task{}}
}

// Run executes a backfill batch for a task and returns the written count. A
// row is written only when its incoming version is not older than the value
// already stored online; rows whose version trails the online value are
// skipped so a backfill of historical data cannot clobber a fresher online
// write. Skipped rows still count toward task progress.
func (b *Backfill) Run(taskID string, version model.Version, rows []Result) (int, error) {
	if taskID == "" {
		return 0, fmt.Errorf("compute: task id must not be empty")
	}
	task := &model.Task{
		ID:        taskID,
		TableID:   version.TableID,
		State:     model.TaskRunning,
		Total:     len(rows),
		UpdatedAt: time.Now(),
	}
	b.tasks[taskID] = task
	written := 0
	for _, key := range BuildPlan(rows).Order {
		row := resultFor(rows, key)
		ok, err := b.store.CompareAndWrite(row.Entity, row.Fields, version)
		if err != nil {
			task.State = model.TaskPartial
			return written, err
		}
		task.Processed++
		if ok {
			written++
		}
		if task.Completed() {
			task.State = model.TaskDone
		}
	}
	task.UpdatedAt = time.Now()
	return written, nil
}

// Task returns the current state of a task.
func (b *Backfill) Task(taskID string) (*model.Task, bool) {
	t, ok := b.tasks[taskID]
	return t, ok
}

// Retry re-runs a failed task. A retry must never write a result that was
// already committed by an earlier attempt.
func (b *Backfill) Retry(taskID string, version model.Version, rows []Result) (int, error) {
	signature := Signature(taskID, rows)
	written := 0
	for _, key := range BuildPlan(rows).Order {
		row := resultFor(rows, key)
		if b.store.Results().IsCommitted(taskID, row.Entity, signature) {
			continue
		}
		ok, err := b.writeResult(row, version)
		if err != nil {
			return written, err
		}
		if ok {
			b.store.Results().MarkCommitted(taskID, row.Entity, signature)
			written++
		}
	}
	return written, nil
}

// writeResult applies an additive update for integer counter fields so a
// computation that is retried accumulates exactly once, and then writes the
// result with a version guard against stale backfill data.
func (b *Backfill) writeResult(row Result, version model.Version) (bool, error) {
	current := b.store.EntryFor(row.Entity)
	if current != nil {
		for name, value := range row.Fields {
			if value.Type != model.FieldInt {
				continue
			}
			if existing, ok := current.Fields[name]; ok && existing.Type == model.FieldInt {
				row.Fields[name] = model.IntValue(existing.Int + value.Int)
			}
		}
	}
	return b.store.CompareAndWrite(row.Entity, row.Fields, version)
}

// resultFor returns the result row for an entity key.
func resultFor(rows []Result, key model.EntityKey) Result {
	for _, row := range rows {
		if row.Entity == key {
			return row
		}
	}
	return Result{}
}
