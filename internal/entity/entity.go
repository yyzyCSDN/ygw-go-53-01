package entity

import (
	"fmt"
	"strings"

	"featurestore/internal/model"
)

// ParseEntity validates and normalizes an entity key before it reaches the
// store or the router.
func ParseEntity(key string) (model.EntityKey, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("entity: empty key")
	}
	if len(key) > 256 {
		return "", fmt.Errorf("entity: key exceeds 256 bytes")
	}
	return model.EntityKey(key), nil
}

// RouteEntity validates an entity key and returns the shard it belongs to
// under the current route table.
func (r *Router) RouteEntity(key model.EntityKey) (int, error) {
	if _, err := ParseEntity(string(key)); err != nil {
		return 0, err
	}
	return r.Route(key), nil
}
