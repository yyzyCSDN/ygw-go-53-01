package table

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is the in-process store of feature table definitions.
type Registry struct {
	mu     sync.RWMutex
	tables map[string]*Table
	order  []string
}

// NewRegistry creates an empty table registry.
func NewRegistry() *Registry {
	return &Registry{tables: map[string]*Table{}}
}

// Register adds or replaces a table definition.
func (r *Registry) Register(t *Table) error {
	if err := t.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tables[t.ID]; !exists {
		r.order = append(r.order, t.ID)
	}
	r.tables[t.ID] = t
	return nil
}

// Get returns a table by identifier.
func (r *Registry) Get(id string) (*Table, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tables[id]
	return t, ok
}

// MustGet returns a table or panics when the table is unknown.
func (r *Registry) MustGet(id string) *Table {
	t, ok := r.Get(id)
	if !ok {
		panic(fmt.Sprintf("unknown feature table %q", id))
	}
	return t
}

// List returns all registered tables ordered by registration time.
func (r *Registry) List() []*Table {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Table, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.tables[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Count returns the number of registered tables.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tables)
}
