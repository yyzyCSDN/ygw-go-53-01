// Package version implements the feature version lifecycle and the active
// version pointer used by the online read path.
package version

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"featurestore/internal/model"
	"featurestore/internal/table"
)

// Manager owns every version of a feature table and the pointer to the
// currently active (published) version. Online reads resolve the active
// version through the manager so a publish becomes visible immediately.
type Manager struct {
	mu       sync.RWMutex
	tables   *table.Registry
	versions map[string]*model.Version
	order    []string
	lastByTable map[string]string
	activeID string
	active   *model.Version
	hook     CacheHook
	now      func() time.Time
}

// CacheHook is implemented by the store to keep the online read cache in
// sync with version transitions.
type CacheHook interface {
	InvalidateVersion(versionID string)
}

// NewManager creates a version manager bound to a table registry.
func NewManager(tables *table.Registry) *Manager {
	return &Manager{
		tables:   tables,
		versions: map[string]*model.Version{},
		lastByTable: map[string]string{},
		now:      time.Now,
	}
}

// SetCacheHook registers the cache invalidation sink.
func (m *Manager) SetCacheHook(hook CacheHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hook = hook
}

// Create drafts a new version for a table.
func (m *Manager) Create(tableID string) (*model.Version, error) {
	if _, ok := m.tables.Get(tableID); !ok {
		return nil, fmt.Errorf("version: unknown table %q", tableID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parent := m.lastByTable[tableID]
	id := fmt.Sprintf("%s-v%d", tableID, len(m.versions)+1)
	definition, _ := m.tables.Get(tableID)
	v := &model.Version{
		ID:        id,
		TableID:   tableID,
		State:     model.VersionDraft,
		ParentID:  parent,
		Fields:    append([]string(nil), definition.FieldNames()...),
		CreatedAt: m.now(),
	}
	m.versions[id] = v
	m.order = append(m.order, id)
	m.lastByTable[tableID] = id
	return v, nil
}

// Publish moves a draft version to published and switches the active
// pointer so subsequent online reads resolve the new version.
func (m *Manager) Publish(id string) (*model.Version, error) {
	m.mu.Lock()
	v, ok := m.versions[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("version: %s not found", id)
	}
	if err := Transition(v, model.VersionPublished); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	v.PublishedAt = m.now()
	if m.active != nil && m.active.ID != id {
		_ = Transition(m.active, model.VersionSuperseded)
	}
	m.activeID = id
	m.mu.Unlock()
	return v, nil
}

// Rollback moves the active version back to its parent and invalidates the
// online cache so cached values of the rolled-back version are dropped.
func (m *Manager) Rollback(id string) (*model.Version, error) {
	m.mu.Lock()
	target, ok := m.versions[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("version: %s not found", id)
	}
	if target.State != model.VersionPublished && target.State != model.VersionSuperseded {
		m.mu.Unlock()
		return nil, fmt.Errorf("version: %s cannot be rolled back from %s", id, target.State)
	}
	parent, ok := m.versions[target.ParentID]
	if !ok || parent.State == model.VersionRolledBack {
		m.mu.Unlock()
		return nil, fmt.Errorf("version: %s has no rollback target", id)
	}
	_ = Transition(target, model.VersionRolledBack)
	_ = Transition(parent, model.VersionPublished)
	m.active = parent
	m.activeID = parent.ID
	hook := m.hook
	m.mu.Unlock()
	if hook != nil {
		hook.InvalidateVersion(id)
		hook.InvalidateVersion(parent.ID)
		if all, ok := hook.(interface{ InvalidateAll() }); ok {
			all.InvalidateAll()
		}
	}
	return parent, nil
}

// ResolveActive returns the currently published version.
func (m *Manager) ResolveActive() *model.Version {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active != nil && !m.active.IsActive() {
		return nil
	}
	return m.active
}

// Get returns a version by identifier.
func (m *Manager) Get(id string) (*model.Version, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id]
	return v, ok
}

// List returns all versions sorted by creation order.
func (m *Manager) List() []*model.Version {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*model.Version, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.versions[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
