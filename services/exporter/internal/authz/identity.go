package authz

import (
	"net/http"
	"slices"

	userdata "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

// Whoever sets a user's mailbox or login identity can sign in as that user, so
// identities are linked only by services and email and username only by their owner.
func guardIdentityFields(w http.ResponseWriter, identity xauthz.Identity, existing *userdata.User, body map[string]any) bool {
	if !guardIdentitiesField(w, existing.Identities, body) {
		return false
	}
	if identity.UserID == existing.ID {
		return true
	}
	if stringFieldChanged(body, constants.FieldEmail, existing.Email) ||
		stringFieldChanged(body, constants.FieldUsername, existing.Username) {
		denyForbidden(w, constants.ErrAuthzIdentityFieldOwnerOnly)
		return false
	}
	return true
}

// Resending the stored list unchanged is allowed, so a form that echoes the record still saves.
func guardIdentitiesField(w http.ResponseWriter, existing []*userdata.UserIdentity, body map[string]any) bool {
	raw, present := body[constants.FieldIdentities]
	if !present {
		return true
	}
	patched, err := sharedutils.ExtractStructFromBody[userdata.User](map[string]any{constants.FieldIdentities: raw})
	if err == nil && slices.EqualFunc(existing, patched.Identities, sameIdentity) {
		return true
	}
	denyForbidden(w, constants.ErrAuthzIdentitiesReserved)
	return false
}

func sameIdentity(a, b *userdata.UserIdentity) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func stringFieldChanged(body map[string]any, field, current string) bool {
	raw, present := body[field]
	if !present {
		return false
	}
	value, isString := raw.(string)
	return !isString || value != current
}
