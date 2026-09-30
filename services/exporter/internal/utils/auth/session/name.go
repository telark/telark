package session

import (
	"net/http"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func SessionName(token string) string {
	return authdata.SessionName(token)
}

func ResolveSessionRef(ref string) string {
	return authdata.SessionRef(ref)
}

// The self session is named by the X-Session-Token header, never by the path: the
// UI sends its raw token and peers send the session name, and SessionRef maps both
// to the name, so neither a token nor its digest reaches a URL or an access log.
func RefFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get(dataconstants.HeaderSessionToken)
	if header == dataconstants.EmptyString {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
			string(constants.ErrSessionSelfRefWithoutToken), nil, nil)
		return dataconstants.EmptyString, false
	}
	ref := authdata.SessionRef(header)
	if ref == authdata.SessionRefSelf {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
			string(constants.ErrSessionRefNotAName), nil, nil)
		return dataconstants.EmptyString, false
	}
	return ref, true
}
