package oidctrust

import (
	"errors"
	"maps"

	"github.com/telark/data/resources/telarkconfig"
	"github.com/telark/exporter/internal/constants"
)

// Removes oidc.googleJwkJson from a config patch so it never reaches the CR; an
// oidc block left empty is dropped with it. null clears the key.
func TakeJWK(spec map[string]any) (value string, present bool, err error) {
	oidc, isMap := spec[telarkconfig.FieldOIDC].(map[string]any)
	if !isMap {
		return constants.EmptyString, false, nil
	}
	raw, present := oidc[telarkconfig.OIDCSecretKey]
	if !present {
		return constants.EmptyString, false, nil
	}
	if raw != nil {
		if value, present = raw.(string); !present {
			return constants.EmptyString, false, errors.New(string(constants.ErrOIDCJWKNotString))
		}
	}
	delete(oidc, telarkconfig.OIDCSecretKey)
	if len(oidc) == constants.DefaultInitValue {
		delete(spec, telarkconfig.FieldOIDC)
	}
	return value, true, nil
}

// The view may share maps with the object it came from, so the oidc block is cloned.
func MergeJWK(view map[string]any, value string) {
	if view == nil || value == constants.EmptyString {
		return
	}
	merged := map[string]any{}
	if oidc, isMap := view[telarkconfig.FieldOIDC].(map[string]any); isMap {
		merged = maps.Clone(oidc)
	}
	merged[telarkconfig.OIDCSecretKey] = value
	view[telarkconfig.FieldOIDC] = merged
}
