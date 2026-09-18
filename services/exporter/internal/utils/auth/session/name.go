package session

import (
	"net/http"

	authdata "github.com/telark/data/auth"
	dataconstants "github.com/telark/data/constants"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
)

func SessionName(token string) string {
	return authdata.SessionName(token)
}

func ResolveSessionRef(ref string) string {
	return authdata.SessionRef(ref)
}

// The ref in the path is a session name or the self ref; self resolves to the
// caller's own token from the header, so a browser never has to derive the
// digest (crypto.subtle is unavailable on plain http) nor put its token in a URL.
// A raw token in the path is refused so it can never reach an access log again.
func RefFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	ref, err := sharedutils.GetPathParam(w, r, constants.TokenParam)
	if err != nil {
		return dataconstants.EmptyString, false
	}
	if authdata.IsSessionName(ref) {
		return ref, true
	}
	if ref != authdata.SessionRefSelf {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
			string(constants.ErrSessionRefNotAName), nil, nil)
		return dataconstants.EmptyString, false
	}
	token := r.Header.Get(dataconstants.HeaderSessionToken)
	if token == dataconstants.EmptyString {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
			string(constants.ErrSessionSelfRefWithoutToken), nil, nil)
		return dataconstants.EmptyString, false
	}
	return token, true
}
