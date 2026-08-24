package model

import "time"

// VersionState is the lifecycle state of a feature table version.
type VersionState string

const (
	VersionDraft      VersionState = "draft"
	VersionPublished  VersionState = "published"
	VersionSuperseded VersionState = "superseded"
	VersionRolledBack VersionState = "rolled-back"
)

// Version describes one immutable generation of a feature table. Each
// version owns its own field layout and write timestamps.
type Version struct {
	ID          string
	TableID     string
	State       VersionState
	ParentID    string
	Fields      []string
	CreatedAt   time.Time
	PublishedAt time.Time
}

// IsActive reports whether the version is currently selectable for reads.
func (v *Version) IsActive() bool {
	return v != nil && v.State == VersionPublished
}
