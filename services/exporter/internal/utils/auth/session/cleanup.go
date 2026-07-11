package session

import (
	"time"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/internal/utils/concurrency"
	"github.com/telark/kcore/crds/api"
)

// PurgeExpiredSessionsForUser deletes expired session CRDs for a user.
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
		if err != nil || session.ExpiresTimestamp == "" {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, session.ExpiresTimestamp)
		if err != nil || !now.After(expiresAt) {
			continue
		}
		name := sessions[i].GetName()
		lock := concurrency.GetLock(name)
		lock.Lock()
		api.DeleteCustomResourceByName(name, authmetadata.UserSessionMetadata)
		lock.Unlock()
	}
}
