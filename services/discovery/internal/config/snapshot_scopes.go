package config

import (
	"github.com/telark/discovery/constants"
)

type ScopeDefinition struct {
	Name       string
	Namespaced bool
}

var snapshotScopes = []ScopeDefinition{
	{Name: constants.DefaultSnapshotScopeDirectory, Namespaced: true},
}

func SnapshotScopes() []ScopeDefinition {
	out := make([]ScopeDefinition, constants.DefaultInitValue, len(snapshotScopes))
	return append(out, snapshotScopes...)
}

func DefaultSnapshotScope() string {
	return snapshotScopes[constants.DefaultInitValue].Name
}
