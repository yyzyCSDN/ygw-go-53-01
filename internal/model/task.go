package model

import "time"

// TaskState is the lifecycle state of a backfill or computation task.
type TaskState string

const (
	TaskPending TaskState = "pending"
	TaskRunning TaskState = "running"
	TaskPartial TaskState = "partial"
	TaskDone    TaskState = "done"
)

// Task describes one backfill computation job and its progress.
type Task struct {
	ID        string
	TableID   string
	State     TaskState
	Total     int
	Processed int
	UpdatedAt time.Time
}

// Completed reports whether every item of the task has been processed.
func (t *Task) Completed() bool {
	return t != nil && t.Processed >= t.Total
}
