// Package store implements the sharded feature storage used by the online
// read path, the offline importer and the backfill engine.
package store

import (
	"sync"
	"time"

	"featurestore/internal/entity"
	"featurestore/internal/model"
	"featurestore/internal/version"
)

// Entry is the mutable state of one feature entity inside a shard.
type Entry struct {
	Key       model.EntityKey
	Fields    map[string]model.FeatureValue
	VersionID string
	WrittenAt time.Time
}

// Clone returns a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	out := &Entry{
		Key:       e.Key,
		VersionID: e.VersionID,
		WrittenAt: e.WrittenAt,
		Fields:    make(map[string]model.FeatureValue, len(e.Fields)),
	}
	for name, value := range e.Fields {
		out.Fields[name] = model.CloneValue(value)
	}
	return out
}

// Snapshot builds an immutable read view of the entry.
func (e *Entry) Snapshot() *model.Snapshot {
	fields := make(map[string]model.FeatureValue, len(e.Fields))
	for name, value := range e.Fields {
		fields[name] = model.CloneValue(value)
	}
	return &model.Snapshot{
		Entity:    e.Key,
		VersionID: e.VersionID,
		Fields:    fields,
		WrittenAt: e.WrittenAt,
	}
}

// Store is the sharded in-memory feature store.
type Store struct {
	mu       sync.RWMutex
	router   *entity.Router
	versions *version.Manager
	cachedActive string
	shards   []map[model.EntityKey]*Entry
	cache    *ReadCache
	journal  *Journal
	results  *CommittedResults
	now      func() time.Time
}

// New creates a store bound to a router and a version manager.
func New(router *entity.Router, versions *version.Manager) *Store {
	count := router.ShardCount()
	shards := make([]map[model.EntityKey]*Entry, count)
	for i := range shards {
		shards[i] = map[model.EntityKey]*Entry{}
	}
	s := &Store{
		router:   router,
		versions: versions,
		shards:   shards,
		cache:    NewReadCache(),
		journal:  NewJournal(),
		results:  NewCommittedResults(),
		now:      time.Now,
	}
	versions.SetCacheHook(s)
	return s
}

// SetClock replaces the time source (used by tests and the demo server).
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// ShardCount returns the number of live shards of the store.
func (s *Store) ShardCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.shards)
}

// ExpandShards grows the store's shard slice to match the router after a
// shard expansion, so writes routed to the new shard range have a place to
// land. Old shards keep their data.
func (s *Store) ExpandShards(newCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if newCount <= len(s.shards) {
		return nil
	}
	grown := make([]map[model.EntityKey]*Entry, newCount)
	copy(grown, s.shards)
	for i := len(s.shards); i < newCount; i++ {
		grown[i] = map[model.EntityKey]*Entry{}
	}
	s.shards = grown
	return nil
}

// shard returns the shard map for a key under the current route table.
func (s *Store) shard(key model.EntityKey) map[model.EntityKey]*Entry {
	index := s.router.Route(key)
	if index < 0 || index >= len(s.shards) {
		index = 0
	}
	return s.shards[index]
}

// lookup returns the live entry for a key or nil when absent.
func (s *Store) lookup(key model.EntityKey) *Entry {
	return s.shard(key)[key]
}

// Read returns the snapshot of an entity under the active version, using the
// online cache when the entry is unchanged since the last read.
func (s *Store) Read(key model.EntityKey) (*model.Snapshot, error) {
	versionID := s.resolveActiveVersionID()
	if cached, ok := s.cache.Get(key, versionID); ok {
		return cached.Clone(), nil
	}
	s.mu.RLock()
	entry := s.shards[s.router.Lookup(key)][key]
	s.mu.RUnlock()
	if entry == nil {
		snap := model.EmptySnapshot(key, versionID)
		s.cache.Put(key, versionID, snap)
		return snap.Clone(), nil
	}
	snap := s.entrySnapshot(entry, versionID)
	s.cache.Put(key, versionID, snap)
	return snap.Clone(), nil
}

// ReadVersion returns the snapshot of an entity for an explicit version.
func (s *Store) ReadVersion(key model.EntityKey, versionID string) (*model.Snapshot, error) {
	if versionID == "" {
		versionID = s.resolveActiveVersionID()
	}
	if cached, ok := s.cache.Get(key, versionID); ok {
		return cached.Clone(), nil
	}
	s.mu.RLock()
	entry := s.shards[s.router.Lookup(key)][key]
	s.mu.RUnlock()
	if entry == nil {
		snap := model.EmptySnapshot(key, versionID)
		s.cache.Put(key, versionID, snap)
		return snap.Clone(), nil
	}
	snap := s.entrySnapshot(entry, versionID)
	s.cache.Put(key, versionID, snap)
	return snap.Clone(), nil
}

// resolveActiveVersionID resolves the current published version through the
// manager. The resolution must happen on every read so a freshly published
// version becomes visible immediately.
func (s *Store) resolveActiveVersionID() string {
	if s.cachedActive != "" {
		return s.cachedActive
	}
	active := s.versions.ResolveActive()
	if active == nil {
		return ""
	}
	s.cachedActive = active.ID
	return s.cachedActive
}

// entrySnapshot builds a snapshot filtered to the field layout of the given
// version. Fields that the version does not know are omitted from the read
// result so a reader never sees columns that did not exist at that version.
func (s *Store) entrySnapshot(entry *Entry, versionID string) *model.Snapshot {
	raw := entry.Snapshot()
	var allowed map[string]bool
	if versionID != "" {
		if v, ok := s.versions.Get(versionID); ok && len(v.Fields) > 0 {
			allowed = make(map[string]bool, len(v.Fields))
			for _, name := range v.Fields {
				allowed[name] = true
			}
		}
	}
	if allowed != nil {
		for name := range raw.Fields {
			if !allowed[name] {
				delete(raw.Fields, name)
			}
		}
	}
	return raw
}

// Write stores the full field set of an entity under the given version.
func (s *Store) Write(key model.EntityKey, fields map[string]model.FeatureValue, versionID string) error {
	return s.WriteAt(key, fields, versionID, s.now())
}

// WriteAt stores an entity with an explicit write timestamp.
func (s *Store) WriteAt(key model.EntityKey, fields map[string]model.FeatureValue, versionID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	shard := s.shard(key)
	entry, ok := shard[key]
	if !ok {
		entry = &Entry{Key: key, Fields: map[string]model.FeatureValue{}}
		shard[key] = entry
	}
	entry.Fields = cloneFields(fields)
	entry.VersionID = versionID
	entry.WrittenAt = at
	s.cache.InvalidateEntity(key)
	return nil
}

// MergeWrite updates only the fields named in the overwrite window and
// preserves every other field, including fields added online while an
// offline import is in progress. A field inside the window that is absent
// from the patch is removed so the imported snapshot is authoritative for
// exactly the fields the file knows about.
func (s *Store) MergeWrite(key model.EntityKey, patch map[string]model.FeatureValue, overwrite []string, versionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	shard := s.shard(key)
	entry, ok := shard[key]
	if !ok {
		entry = &Entry{Key: key, Fields: map[string]model.FeatureValue{}}
		shard[key] = entry
	}
	if entry.Fields == nil {
		entry.Fields = map[string]model.FeatureValue{}
	}
	for _, name := range overwrite {
		if value, present := patch[name]; present {
			entry.Fields[name] = model.CloneValue(value)
		} else {
			delete(entry.Fields, name)
		}
	}
	entry.VersionID = versionID
	entry.WrittenAt = s.now()
	s.cache.InvalidateEntity(key)
	return nil
}

// CompareAndWrite writes an entity only when the incoming version is not
// older than the currently stored version; stale writes are rejected.
func (s *Store) CompareAndWrite(key model.EntityKey, fields map[string]model.FeatureValue, incoming model.Version) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.lookup(key)
	if entry != nil && entry.VersionID != "" && versionNewer(entry.VersionID, incoming.ID) {
		return false, nil
	}
	shard := s.shard(key)
	if entry == nil {
		entry = &Entry{Key: key, Fields: map[string]model.FeatureValue{}}
		shard[key] = entry
	}
	entry.Fields = cloneFields(fields)
	entry.VersionID = incoming.ID
	entry.WrittenAt = s.now()
	s.cache.InvalidateEntity(key)
	return true, nil
}

// Delete removes an entity from the store.
func (s *Store) Delete(key model.EntityKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.shard(key), key)
	s.cache.InvalidateEntity(key)
	return nil
}

// WrittenAt returns the last write time of an entity for TTL scanning.
func (s *Store) WrittenAt(key model.EntityKey) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.lookup(key)
	if entry == nil {
		return time.Time{}, false
	}
	return entry.WrittenAt, true
}

// EntryFor returns a deep copy of the live entry (nil when absent).
func (s *Store) EntryFor(key model.EntityKey) *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.lookup(key)
	if entry == nil {
		return nil
	}
	return entry.Clone()
}

// Keys returns the entity keys of one shard.
func (s *Store) Keys(shardIndex int) []model.EntityKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if shardIndex < 0 || shardIndex >= len(s.shards) {
		return nil
	}
	out := make([]model.EntityKey, 0, len(s.shards[shardIndex]))
	for key := range s.shards[shardIndex] {
		out = append(out, key)
	}
	return out
}

// InvalidateVersion drops cached reads for a version.
func (s *Store) InvalidateVersion(versionID string) {
	s.cache.InvalidateVersion(versionID)
}

// Journal exposes the write journal used by import rollback.
func (s *Store) Journal() *Journal {
	return s.journal
}

// RollbackBatch reverts every journaled write of a batch, including writes
// from segments that were still in flight, and clears the journal.
func (s *Store) RollbackBatch(batchID string) error {
	ops := s.journal.OpsFor(batchID)
	seen := map[model.EntityKey]bool{}
	for _, op := range ops {
		if seen[op.Key] {
			continue
		}
		seen[op.Key] = true
		if op.Before == nil {
			if err := s.Delete(op.Key); err != nil {
				return err
			}
			continue
		}
		if err := s.WriteAt(op.Key, op.Before.Fields, op.Before.VersionID, op.Before.WrittenAt); err != nil {
			return err
		}
	}
	s.journal.Clear(batchID)
	return nil
}

// Results exposes the committed-result registry used by compute retries.
func (s *Store) Results() *CommittedResults {
	return s.results
}

// InvalidateAll drops every cached read; called by the version manager after
// a rollback so no stale value survives the transition.
func (s *Store) InvalidateAll() {
	s.cache.ClearAll()
}

func cloneFields(fields map[string]model.FeatureValue) map[string]model.FeatureValue {
	out := make(map[string]model.FeatureValue, len(fields))
	for name, value := range fields {
		out[name] = model.CloneValue(value)
	}
	return out
}

func versionNewer(current, incoming string) bool {
	return versionRank(current) > versionRank(incoming)
}

// versionRank extracts the numeric rank of a version id so ordering follows
// the publish sequence instead of a lexicographic string comparison.
func versionRank(id string) int {
	rank := 0
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] < '0' || id[i] > '9' {
			break
		}
		rank = rank*10 + int(id[i]-'0')
	}
	return rank
}
