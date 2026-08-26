package version

import (
	"fmt"

	"featurestore/internal/model"
)

// transitions defines the allowed version state machine edges.
var transitions = map[model.VersionState]map[model.VersionState]bool{
	model.VersionDraft: {
		model.VersionPublished: true,
	},
	model.VersionPublished: {
		model.VersionSuperseded: true,
		model.VersionRolledBack: true,
	},
	model.VersionSuperseded: {
		model.VersionRolledBack: true,
	},
	model.VersionRolledBack: {},
}

// CanTransition reports whether the state machine allows from -> to.
func CanTransition(from, to model.VersionState) bool {
	edges, ok := transitions[from]
	if !ok {
		return false
	}
	return edges[to]
}

// Transition validates and applies a state change on a version record.
func Transition(v *model.Version, to model.VersionState) error {
	if v == nil {
		return fmt.Errorf("version: nil record")
	}
	if !CanTransition(v.State, to) {
		return fmt.Errorf("version: invalid transition %s -> %s", v.State, to)
	}
	v.State = to
	return nil
}
