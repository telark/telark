package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"os"
	"strings"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/services/exporter/internal/constants"
)

// Redis needs no credentials to reach, so whatever it holds is attacker
// controllable. Cache entries that drive authorization are signed with the
// service token — which lives in a secret and never enters Redis — so a planted
// entry fails verification instead of granting permissions.
func SignCacheEntry(binding, payload string) (string, bool) {
	mac, ok := cacheMAC(binding, payload)
	if !ok {
		return constants.EmptyString, false
	}
	return mac + constants.CacheSignatureSeparator + payload, true
}

// The binding ties a signature to the thing it was issued for, so a valid entry
// cannot be lifted from one user's key onto another's.
func VerifyCacheEntry(binding, entry string) (string, bool) {
	mac, payload, found := strings.Cut(entry, constants.CacheSignatureSeparator)
	if !found {
		return constants.EmptyString, false
	}

	expected, ok := cacheMAC(binding, payload)
	if !ok {
		return constants.EmptyString, false
	}

	if subtle.ConstantTimeCompare([]byte(mac), []byte(expected)) != constants.ConstantTimeEqual {
		return constants.EmptyString, false
	}

	return payload, true
}

// Without a key nothing can be signed or verified, so the caller treats the
// cache as unusable rather than trusting it blindly.
func cacheMAC(binding, payload string) (string, bool) {
	key := strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken))
	if key == constants.EmptyString {
		return constants.EmptyString, false
	}

	// The binding is folded to a fixed width first: concatenating it with the
	// payload would let a binding ending in the separator stand in for a longer
	// one, so two different pairs could share a signature.
	bound := sha256.Sum256([]byte(constants.CacheSignatureLabel + binding))

	digest := hmac.New(sha256.New, []byte(key))
	_, _ = digest.Write(bound[:])
	_, _ = digest.Write([]byte(payload))

	return hex.EncodeToString(digest.Sum(nil)), true
}
