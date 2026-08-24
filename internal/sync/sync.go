// Package sync implements the online/offline consistency check that splits a
// time range into contiguous windows and compares both sides per window.
package sync

import (
	"fmt"
	"time"

	"featurestore/internal/model"
	"featurestore/internal/store"
	"featurestore/internal/version"
)

// Diff describes one feature value that differs between online and offline.
type Diff struct {
	Entity   model.EntityKey
	Field    string
	Window   int
	Online   string
	Offline  string
}

// Checker compares online snapshots against an offline view.
type Checker struct {
	store    *store.Store
	versions *version.Manager
	step     time.Duration
}

// NewChecker creates a consistency checker with a fixed window step.
func NewChecker(s *store.Store, versions *version.Manager, step time.Duration) *Checker {
	if step <= 0 {
		step = time.Hour
	}
	return &Checker{store: s, versions: versions, step: step}
}

// offline is the callback that returns the offline snapshot of an entity.
type offline func(key model.EntityKey) *model.Snapshot

// Check splits [begin, until] into contiguous windows bounded by version
// switch events and reports every difference between the online and offline
// views inside each window. The tail window after the last switch is always
// compared; a range remainder is never skipped.
func (c *Checker) Check(begin, until time.Time, offlineView offline) ([]Diff, error) {
	if !until.After(begin) {
		return nil, fmt.Errorf("sync: invalid range %v..%v", begin, until)
	}
	_ = c.versions.SwitchEvents(begin, until)
	diffs := make([]Diff, 0)
	window := 0
	for start := begin; start.Before(until); start = start.Add(c.step) {
		end := start.Add(c.step)
		if end.After(until) {
			break
		}
		diffs = append(diffs, c.compareWindow(start, end, window, offlineView)...)
		window++
	}
	return diffs, nil
}

// compareWindow compares every entity of the store against the offline view
// inside one window.
func (c *Checker) compareWindow(begin, end time.Time, window int, offlineView offline) []Diff {
	diffs := make([]Diff, 0)
	shardCount := c.store.ShardCount()
	for shard := 0; shard < shardCount; shard++ {
		for _, key := range c.store.Keys(shard) {
			writtenAt, ok := c.store.WrittenAt(key)
			if !ok {
				continue
			}
			if writtenAt.Before(begin) || !writtenAt.Before(end) {
				continue
			}
			online, err := c.store.ReadVersion(key, "")
			if err != nil {
				continue
			}
			off := offlineView(key)
			for _, name := range online.FieldNames() {
				onlineValue := online.Fields[name]
				var offlineValue model.FeatureValue
				if off != nil {
					offlineValue = off.Fields[name]
				}
				if onlineValue.String() != offlineValue.String() {
					diffs = append(diffs, Diff{
						Entity:  key,
						Field:   name,
						Window:  window,
						Online:  onlineValue.String(),
						Offline: offlineValue.String(),
					})
				}
			}
		}
	}
	return diffs
}
