package passkey

import (
	"net/http"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func PatchPasskeyByCredentialID(w http.ResponseWriter, credentialID string, userID string, patchData map[string]any) {
	resource, err := passkeyutils.FindPasskeyByCredentialIDAndUserID(credentialID, userID)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}

	passkeyName := resource.GetName()

	filteredPatchData, err := passkeyutils.ExtractPatchFields(patchData)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}

	specPatchData := map[string]any{
		constants.SpecField: filteredPatchData,
	}

	lock := concurrency.GetLock(passkeyName)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericPatchCustomResource(w, authmetadata.PasskeyMetadata, passkeyName, specPatchData)
}
