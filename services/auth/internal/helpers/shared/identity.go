package shared

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/telark/auth/internal/constants"
)

// Stable, non-reversible handle: log lines stay correlatable per subject
// without the email or user id itself reaching the log.
func IdentityHash(identifier string) string {
	if identifier == constants.EmptyString {
		return constants.IdentityHashUnknown
	}
	sum := sha256.Sum256([]byte(identifier))
	return hex.EncodeToString(sum[:])[:constants.IdentityHashLength]
}
