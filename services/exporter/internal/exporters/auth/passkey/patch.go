package passkey

import (
	"net/http"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/exporters/generics"
	passkeyutils "github.com/telark/exporter/utils/auth/passkey"
	"github.com/telark/exporter/utils/concurrency"
	sharedutils "github.com/telark/exporter/utils/shared"
)

func PatchPasskeyByCredentialID(w http.ResponseWriter, credentialID string, userID string, patchData map[string]any) {
	// Find passkey by credentialId and verify it belongs to the user
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

	generics.GenericPatchCustomResource(w, authmetadata.UserPasskeyMetadata, passkeyName, specPatchData)
}
