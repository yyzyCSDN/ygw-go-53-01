package entity

import (
	"fmt"
	"sync"

	"featurestore/internal/model"
)

// Router maps entity keys to shards. The route table mirrors the current
// shard count so a write and a read of the same key always agree on the
// destination shard.
type Router struct {
	mu          sync.RWMutex
	shardCount  int
	routeTable  []int
	shardStates []int
}

// NewRouter creates a router with the given initial shard count.
func NewRouter(shardCount int) (*Router, error) {
	if shardCount <= 0 {
		return nil, fmt.Errorf("entity: shard count must be positive")
	}
	r := &Router{shardCount: shardCount}
	r.rebuildTable(shardCount)
	return r, nil
}

// ShardCount returns the current number of shards.
func (r *Router) ShardCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.shardCount
}

// Route returns the shard index for an entity key under the current route
// table. Both writes and reads must use this method.
func (r *Router) Route(key model.EntityKey) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	slot := Slot(key, len(r.routeTable))
	return r.routeTable[slot]
}

// Lookup is the read-path alias of Route; keeping both names documents that
// reads must honour the same table as writes.
func (r *Router) Lookup(key model.EntityKey) int {
	return r.Route(key)
}

// ExpandShards grows the router to the new shard count and rebuilds the
// route table so newly written entities are placed on the expanded set.
func (r *Router) ExpandShards(newCount int) error {
	if newCount <= 0 {
		return fmt.Errorf("entity: shard count must be positive")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if newCount == r.shardCount {
		return nil
	}
	if newCount < r.shardCount {
		return fmt.Errorf("entity: cannot shrink from %d to %d", r.shardCount, newCount)
	}
	r.rebuildTable(newCount)
	r.shardCount = newCount
	return nil
}

// rebuildTable initialises the route table so every slot maps to its own
// shard index.
func (r *Router) rebuildTable(count int) {
	table := make([]int, count)
	states := make([]int, count)
	for i := 0; i < count; i++ {
		table[i] = i
		states[i] = 0
	}
	r.routeTable = table
	r.shardStates = states
}

// ShardState returns the free/total marker of a shard (used by the browse
// page to show expansion progress).
func (r *Router) ShardState(shard int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if shard < 0 || shard >= len(r.shardStates) {
		return -1
	}
	return r.shardStates[shard]
}
