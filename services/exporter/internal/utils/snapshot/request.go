package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	derrs "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/exporter/internal/utils/artifact"
	restsnapshot "github.com/telark/rest/clients/snapshots"
)

func ParseCreateSnapshotRequest(body map[string]any) (*restsnapshot.CreateSnapshotPayload, error) {
	id, err := parseRequiredTrimmedString(body, constants.FieldID, constants.ErrSnapshotIDSimpleRequired)
	if err != nil {
		return nil, err
	}
	scope, err := parseRequiredTrimmedString(body, constants.FieldScope, constants.ErrSnapshotScopeSimpleRequired)
	if err != nil {
		return nil, err
	}
	namespace := parseOptionalNamespace(body)
	scopes := envmanager.GetSnapshotScopes()
	if err := validateScopeAndNamespace(scope, namespace, scopes); err != nil {
		return nil, err
	}
	gen, err := parseGeneration(body)
	if err != nil {
		return nil, err
	}
	manifest, err := parseManifest(body)
	if err != nil {
		return nil, err
	}

	return &restsnapshot.CreateSnapshotPayload{
		ID:         id,
		Scope:      scope,
		Namespace:  namespace,
		Generation: gen,
		Manifest:   manifest,
	}, nil
}

func parseRequiredTrimmedString(
	body map[string]any,
	field string,
	validationErr derrs.Error,
) (string, error) {
	raw, exists := body[field]
	value, typeOK := raw.(string)
	value = strings.TrimSpace(value)
	if !exists || !typeOK || value == constants.EmptyString {
		return constants.EmptyString, errors.New(string(validationErr))
	}

	return value, nil
}

func parseOptionalNamespace(body map[string]any) string {
	namespace, ok := body[constants.FieldNamespace].(string)
	if !ok {
		return constants.EmptyString
	}

	return strings.TrimSpace(namespace)
}

func validateScopeAndNamespace(scope string, namespace string, scopes []envmanager.ScopeDefinition) error {
	if !envmanager.IsScopeValid(scopes, scope) {
		return fmt.Errorf(string(constants.ErrSnapshotScopeNotRegistered), scope, RegisteredScopesText(scopes))
	}
	if envmanager.IsScopeNamespaced(scopes, scope) && namespace == constants.EmptyString {
		return fmt.Errorf(string(constants.ErrSnapshotNamespaceRequired), scope)
	}

	return nil
}

func parseGeneration(body map[string]any) (int, error) {
	genRaw, genExists := body[constants.FieldGeneration]
	if !genExists {
		return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotGenerationRequired))
	}

	gen, err := toPositiveInt(genRaw)
	if err != nil {
		return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotGenerationRequired))
	}

	return gen, nil
}

func parseManifest(body map[string]any) (any, error) {
	manifest, exists := body[constants.FieldManifest]
	if !exists || manifest == nil {
		return nil, errors.New(string(constants.ErrSnapshotManifestSimpleRequired))
	}

	return manifest, nil
}

func ResolveTarget(id string, scope string, namespace string, generation string) (*SnapshotTarget, error) {
	if err := ValidateSnapshotIdentity(id, scope); err != nil {
		return nil, err
	}
	scopes := envmanager.GetSnapshotScopes()
	if !envmanager.IsScopeValid(scopes, scope) {
		return nil, fmt.Errorf(string(constants.ErrSnapshotScopeNotRegistered), scope, RegisteredScopesText(scopes))
	}

	return ResolveSnapshotPath(
		envmanager.GetSnapshotsPath(),
		scope,
		id,
		namespace,
		generation,
		envmanager.IsScopeNamespaced(scopes, scope),
	)
}

func RegisteredScopeNames(scopes []envmanager.ScopeDefinition) []string {
	names := make([]string, constants.DefaultInitValue, len(scopes))
	for _, scope := range scopes {
		names = append(names, scope.Name)
	}
	return names
}

func RegisteredScopesText(scopes []envmanager.ScopeDefinition) string {
	return strings.Join(RegisteredScopeNames(scopes), ",")
}

func SnapshotNotFoundMessage(id string, scope string, namespace string, generation string) string {
	if strings.TrimSpace(generation) == constants.EmptyString {
		generation = latestGenerationValue
	}

	return fmt.Sprintf(
		string(constants.ErrSnapshotNotFoundByTarget),
		id,
		scope,
		namespace,
		generation,
	)
}

func ContentDispositionFilename(id string, generation int) string {
	fileName := fmt.Sprintf("%s-G%d%s", id, generation, constants.SnapshotRollbackFilenameSuffix)

	return fmt.Sprintf(constants.ContentDispositionAttachmentTemplate, fileName)
}

func LoadSnapshotData(path string) (map[string]any, error) {
	cleaned := filepath.Clean(path)
	if !artifact.IsWithinBase(cleaned, envmanager.GetSnapshotsPath()) {
		return nil, os.ErrNotExist
	}
	content, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func toPositiveInt(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		if n < constants.SnapshotGenerationMinValue || n != float64(int64(n)) {
			return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotInvalidInt))
		}
		return int(n), nil
	case int:
		if n < constants.SnapshotGenerationMinValue {
			return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotInvalidInt))
		}
		return n, nil
	case int64:
		if n < constants.SnapshotGenerationMinValue {
			return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotInvalidInt))
		}
		return int(n), nil
	case string:
		n = strings.TrimSpace(n)
		parsed, err := strconv.Atoi(n)
		if err != nil || parsed < constants.SnapshotGenerationMinValue {
			return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotInvalidInt))
		}
		return parsed, nil
	default:
		return constants.DefaultInitValue, errors.New(string(constants.ErrSnapshotInvalidInt))
	}
}
