package store

import (
	"sync"

	"featurestore/internal/model"
)

// cacheEntry is one cached read snapshot keyed by entity and version.
type cacheEntry struct {
	key       string
	snapshot  *model.Snapshot
}

// ReadCache is the online read cache. Version transitions invalidate it so
// a publish or rollback is never masked by a stale cached snapshot.
type ReadCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

// NewReadCache creates an empty read cache.
func NewReadCache() *ReadCache {
	return &ReadCache{entries: map[string]*cacheEntry{}}
}

func cacheKey(entity model.EntityKey, versionID string) string {
	return string(entity) + "@" + versionID
}

// Get returns the cached snapshot for an entity/version pair.
func (c *ReadCache) Get(entity model.EntityKey, versionID string) (*model.Snapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[cacheKey(entity, versionID)]
	if !ok {
		return nil, false
	}
	return entry.snapshot, true
}

// Put stores a snapshot for an entity/version pair.
func (c *ReadCache) Put(entity model.EntityKey, versionID string, snapshot *model.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[cacheKey(entity, versionID)] = &cacheEntry{
		key:      cacheKey(entity, versionID),
		snapshot: snapshot.Clone(),
	}
}

// InvalidateVersion removes every cached read of the given version.
func (c *ReadCache) InvalidateVersion(versionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := "@" + versionID
	for key, entry := range c.entries {
		if len(entry.key) >= len(prefix) && entry.key[len(entry.key)-len(prefix):] == prefix {
			delete(c.entries, key)
		}
	}
}

// InvalidateEntity removes every cached read of the given entity.
func (c *ReadCache) InvalidateEntity(entity model.EntityKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := string(entity) + "@"
	for key, entry := range c.entries {
		if len(entry.key) >= len(prefix) && entry.key[:len(prefix)] == prefix {
			delete(c.entries, key)
		}
	}
}

// Len returns the number of cached entries.
func (c *ReadCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// ClearAll drops every cached snapshot.
func (c *ReadCache) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]*cacheEntry{}
}
