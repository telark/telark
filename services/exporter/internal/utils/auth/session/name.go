package session

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/telark/exporter/internal/constants"
)

// The token is never persisted; its digest names the resource instead. That
// keeps the credential out of etcd and still resolves a token in one lookup.
func SessionName(token string) string {
	digest := sha256.Sum256([]byte(token))
	return constants.SessionNamePrefix + hex.EncodeToString(digest[:])
}

// Endpoints that address one session accept either its token or its resource
// name, so the UI can revoke a device without ever being handed that device's
// token. Only the authentication path is restricted to tokens: a name is not a
// credential, and SessionName is what enforces that.
func ResolveSessionRef(ref string) string {
	if isSessionName(ref) {
		return ref
	}
	return SessionName(ref)
}

func isSessionName(ref string) bool {
	digest, found := strings.CutPrefix(ref, constants.SessionNamePrefix)
	if !found || len(digest) != constants.SessionDigestLength {
		return false
	}

	_, err := hex.DecodeString(digest)
	return err == nil
}
