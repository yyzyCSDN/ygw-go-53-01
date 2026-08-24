// Package entity implements entity-key hashing and the shard routing table
// used by the feature store to place and locate entities.
package entity

import (
	"github.com/cespare/xxhash/v2"

	"featurestore/internal/model"
)

// HashKey returns the 64-bit hash of an entity key.
func HashKey(key model.EntityKey) uint64 {
	return xxhash.Sum64String(string(key))
}

// Slot returns the routing slot of a key for a given table size.
func Slot(key model.EntityKey, size int) int {
	if size <= 0 {
		return 0
	}
	return int(HashKey(key) % uint64(size))
}
