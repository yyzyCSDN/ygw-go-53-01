// Package ttl implements time-to-live expiration for feature entries.
package ttl

import (
	"time"

	"featurestore/internal/model"
	"featurestore/internal/store"
)

// Scanner expires feature entries whose age is strictly older than the
// configured TTL. Entries written exactly on the expiry boundary are kept so
// a fresh write is never removed by the same scan that observes it.
type Scanner struct {
	store *store.Store
	ttl   time.Duration
}

// NewScanner creates a TTL scanner.
func NewScanner(s *store.Store, ttl time.Duration) *Scanner {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Scanner{store: s, ttl: ttl}
}

// Scan removes every entry older than the TTL at the given time and returns
// the number of entries expired.
func (sc *Scanner) Scan(now time.Time) int {
	expired := 0
	shardCount := sc.store.ShardCount()
	for shard := 0; shard < shardCount; shard++ {
		for _, key := range sc.store.Keys(shard) {
			if sc.ExpireKey(key, now) {
				expired++
			}
		}
	}
	return expired
}

// IsExpired reports whether an entry of the given age is past the TTL. An
// entry whose age equals the TTL is treated as still live: expiry is strict,
// so a feature written exactly on the expiry boundary survives the scan that
// observes it and is never mistaken for stale data.
func (sc *Scanner) IsExpired(age time.Duration) bool {
	return age > sc.ttl
}

// ExpireKey is a direct expiry helper used by the HTTP handler.
func (sc *Scanner) ExpireKey(key model.EntityKey, now time.Time) bool {
	writtenAt, ok := sc.store.WrittenAt(key)
	if !ok {
		return false
	}
	if !sc.IsExpired(now.Sub(writtenAt)) {
		return false
	}
	_ = sc.store.Delete(key)
	return true
}
