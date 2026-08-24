package compute

import (
	"sort"

	"featurestore/internal/model"
)

// Plan is a deterministic execution order of a backfill batch.
type Plan struct {
	Order []model.EntityKey
}

// BuildPlan sorts the result entities so a retry produces the same sequence.
func BuildPlan(rows []Result) Plan {
	keys := make([]model.EntityKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Entity)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return Plan{Order: keys}
}

// Signature derives a stable fingerprint of a result set.
func Signature(taskID string, rows []Result) string {
	total := 0
	for _, row := range rows {
		for name, value := range row.Fields {
			total += len(name) + len(value.String())
		}
	}
	return fmtSignature(taskID, total)
}
