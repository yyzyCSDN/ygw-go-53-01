package version

import (
	"sort"
	"time"

	"featurestore/internal/model"
)

// SwitchEvent describes one version transition that the consistency checker
// uses to split its comparison windows.
type SwitchEvent struct {
	VersionID string
	At        time.Time
}

// SwitchEvents returns the publish times of every published version inside
// [since, until), ordered by time, followed by a sentinel boundary at
// "until" so the caller always knows where the final window ends. The
// returned boundaries are contiguous by construction.
func (m *Manager) SwitchEvents(since, until time.Time) []SwitchEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	events := make([]SwitchEvent, 0)
	for _, v := range m.versions {
		if v.State == model.VersionPublished && !v.PublishedAt.IsZero() {
			if !v.PublishedAt.Before(since) && v.PublishedAt.Before(until) {
				events = append(events, SwitchEvent{VersionID: v.ID, At: v.PublishedAt})
			}
		}
	}
	events = append(events, SwitchEvent{At: until})
	sort.Slice(events, func(i, j int) bool {
		if events[i].At.Equal(events[j].At) {
			return events[i].VersionID < events[j].VersionID
		}
		return events[i].At.Before(events[j].At)
	})
	return events
}
