package shared

import (
	"net/http"
	"time"

	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func AddLastUpdateDateToPatchBody(body map[string]any) {
	if body == nil {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	body["lastUpdateDate"] = now
}

// Audit actors are the authenticated caller, never the body; a service call
// that names no user leaves the stored actor as it was.
func StampPatchAudit(r *http.Request, body map[string]any) {
	delete(body, constants.FieldCreatedBy)
	delete(body, constants.FieldLastUpdatedBy)
	if identity, _ := xauthz.FromContext(r.Context()); identity.UserID != constants.EmptyString {
		body[constants.FieldLastUpdatedBy] = identity.UserID
	}
}

func StampCreateAudit(r *http.Request, body map[string]any) {
	StampPatchAudit(r, body)
	if actor, stamped := body[constants.FieldLastUpdatedBy]; stamped {
		body[constants.FieldCreatedBy] = actor
	}
}
