package session

import (
	"errors"
	"net/http"

	authmetadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	authutils "github.com/telark/exporter/internal/utils/auth/shared"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

func PatchSessionByToken(w http.ResponseWriter, token string, patchData map[string]any) {
	resource, ok := sessionutils.FindSessionOrRespond(w, token)
	if !ok {
		return
	}

	sessionName := resource.GetName()

	expiresTimestamp, err := validateAndExtractExpiresTimestamp(patchData)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}

	filteredPatchData := map[string]any{
		constants.FieldExpiresTimestamp: expiresTimestamp,
	}

	specPatchData := map[string]any{
		constants.SpecField: filteredPatchData,
	}

	lock := concurrency.GetLock(sessionName)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericPatchCustomResource(w, authmetadata.SessionMetadata, sessionName, specPatchData)
}

func validateAndExtractExpiresTimestamp(patchData map[string]any) (string, error) {
	expiresTimestamp, hasExpiresTimestamp := patchData[constants.FieldExpiresTimestamp].(string)
	if !hasExpiresTimestamp {
		return constants.EmptyString, errors.New(string(constants.ErrSessionPatchOnlyExpiresTimestamp))
	}

	if len(patchData) > constants.DefaultChannelBufferSize {
		return constants.EmptyString, errors.New(string(constants.ErrSessionPatchOnlyExpiresTimestamp))
	}

	if expiresTimestamp != constants.EmptyString {
		if err := authutils.ValidateExpiresAt(expiresTimestamp, constants.ErrSessionExpiresInPast); err != nil {
			return constants.EmptyString, err
		}
	}

	return expiresTimestamp, nil
}
