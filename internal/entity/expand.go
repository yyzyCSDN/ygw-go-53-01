package entity

import (
	"fmt"
)

// Expansion records the outcome of a shard resize operation.
type Expansion struct {
	Previous int
	Current  int
	Added    int
}

// Expand is a convenience wrapper used by the HTTP handler: it resizes the
// router and reports how many shards were added.
func (r *Router) Expand(target int) (Expansion, error) {
	before := r.ShardCount()
	if err := r.ExpandShards(target); err != nil {
		return Expansion{}, err
	}
	after := r.ShardCount()
	if after <= before {
		return Expansion{}, fmt.Errorf("entity: expansion did not change shard count")
	}
	return Expansion{Previous: before, Current: after, Added: after - before}, nil
}
