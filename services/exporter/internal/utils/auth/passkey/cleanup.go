package passkey

import (
	"errors"
	"net/http"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
)

func PurgePasskeysForUser(userID string) error {
	passkeys, err := FindPasskeysByUserID(userID)
	if err != nil {
		return err
	}
	errs := make([]error, constants.DefaultInitValue, len(passkeys))
	for i := range passkeys {
		name := passkeys[i].GetName()
		lock := concurrency.GetLock(name)
		lock.Lock()
		result := api.DeleteCustomResourceByName(name, authmetadata.PasskeyMetadata)
		lock.Unlock()
		if result.Status != http.StatusOK && result.Status != http.StatusNotFound {
			errs = append(errs, result.Error)
		}
	}
	return errors.Join(errs...)
}
