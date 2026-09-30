package session

import (
	"errors"
	"net/http"
	"time"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	kshared "github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
)

// Called before creating a new session to keep the CRD count bounded,
// preventing Kubernetes ResourceQuota evaluation timeouts on creation.
func PurgeExpiredSessionsForUser(userID string) {
	sessions, err := FindSessionsByUserID(userID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for i := range sessions {
		session, err := UnstructuredToSession(&sessions[i])
		if err != nil || session.ExpiresTimestamp == constants.EmptyString {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, session.ExpiresTimestamp)
		if err != nil || !now.After(expiresAt) {
			continue
		}
		deleteSession(sessions[i].GetName())
	}
}

func PurgeSessionsForUser(userID string) error {
	sessions, err := FindSessionsByUserID(userID)
	if err != nil {
		return err
	}
	errs := make([]error, constants.DefaultInitValue, len(sessions))
	for i := range sessions {
		if result := deleteSession(sessions[i].GetName()); result.Status != http.StatusOK && result.Status != http.StatusNotFound {
			errs = append(errs, result.Error)
		}
	}
	return errors.Join(errs...)
}

func deleteSession(name string) kshared.KubernetesAPIData {
	lock := concurrency.GetLock(name)
	lock.Lock()
	defer lock.Unlock()
	return api.DeleteCustomResourceByName(name, authmetadata.SessionMetadata)
}
