package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	SessionNamePrefix   = "session-"
	SessionDigestLength = 64
	// Path ref a caller may use for the session its own X-Session-Token names.
	SessionRefSelf = "self"
)

// The token is never persisted and never travels in a URL: its digest names the resource, so
// lookups, path parameters and access logs only ever see the name.
func SessionName(token string) string {
	digest := sha256.Sum256([]byte(token))
	return SessionNamePrefix + hex.EncodeToString(digest[:])
}

func IsSessionName(ref string) bool {
	digest, found := strings.CutPrefix(ref, SessionNamePrefix)
	if !found || len(digest) != SessionDigestLength {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

// A name or the self ref passes through unchanged; a raw token is hashed to its name.
func SessionRef(ref string) string {
	if ref == SessionRefSelf || IsSessionName(ref) {
		return ref
	}
	return SessionName(ref)
}
