package snapshot

import (
	"errors"
	"strconv"
	"strings"

	"github.com/telark/exporter/internal/constants"
	restsnapshot "github.com/telark/rest/clients/snapshots"
)

func ParseCreatePayload(body map[string]any) (*restsnapshot.CreateSnapshotPayload, error) {
	idValue, ok := body[constants.FieldID]
	if !ok {
		return nil, errors.New(string(constants.ErrSnapshotIDRequired))
	}
	id, ok := idValue.(string)
	if !ok {
		return nil, errors.New(string(constants.ErrSnapshotIDRequired))
	}
	if id == constants.EmptyString {
		return nil, errors.New(string(constants.ErrSnapshotIDRequired))
	}

	scopeValue, ok := body[constants.FieldScope]
	if !ok {
		return nil, errors.New(string(constants.ErrSnapshotScopeRequired))
	}
	scope, ok := scopeValue.(string)
	if !ok {
		return nil, errors.New(string(constants.ErrSnapshotScopeRequired))
	}
	if scope == constants.EmptyString {
		return nil, errors.New(string(constants.ErrSnapshotScopeRequired))
	}

	manifest, exists := body[constants.FieldManifest]
	if !exists || manifest == nil {
		return nil, errors.New(string(constants.ErrSnapshotManifestRequired))
	}

	namespaceValue := body[constants.FieldNamespace]
	namespace, namespaceOK := namespaceValue.(string)
	if !namespaceOK {
		namespace = constants.EmptyString
	}
	generation, err := requiredPositiveInt(body, constants.FieldGeneration)
	if err != nil {
		return nil, err
	}

	return &restsnapshot.CreateSnapshotPayload{
		ID:         id,
		Scope:      scope,
		Namespace:  strings.TrimSpace(namespace),
		Generation: generation,
		Manifest:   manifest,
	}, nil
}

func requiredPositiveInt(body map[string]any, key string) (int, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return constants.DefaultInitValue, errors.New(generationRequiredMsg)
	}
	n, err := coerceInt(raw)
	if err != nil || n < constants.DefaultIncrementValue {
		return constants.DefaultInitValue, errors.New(generationRequiredMsg)
	}

	return n, nil
}

func coerceInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		if n < float64(constants.DefaultIncrementValue) || n != float64(int64(n)) {
			return constants.DefaultInitValue, errors.New(generationRequiredMsg)
		}

		return int(n), nil
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))

		return i, err
	default:
		return constants.DefaultInitValue, errors.New(generationRequiredMsg)
	}
}
