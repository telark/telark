package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"os"
	"strings"

	dataconstants "github.com/telark/telark/internal/data/constants"
)

// Every service holds the Redis password, so whatever Redis holds is attacker
// controllable. Entries that drive authorization are signed with the service
// token — which lives in a secret and never enters Redis — so a planted entry
// fails verification instead of granting anything.
func SignRedisEntry(binding, payload string) (string, bool) {
	mac, ok := entryMAC(binding, payload)
	if !ok {
		return dataconstants.EmptyString, false
	}
	return mac + signatureSeparator + payload, true
}

// The binding ties a signature to the thing it was issued for, so a valid entry
// cannot be lifted from one key onto another.
func VerifyRedisEntry(binding, entry string) (string, bool) {
	mac, payload, found := strings.Cut(entry, signatureSeparator)
	if !found {
		return dataconstants.EmptyString, false
	}

	expected, ok := entryMAC(binding, payload)
	if !ok {
		return dataconstants.EmptyString, false
	}

	if subtle.ConstantTimeCompare([]byte(mac), []byte(expected)) != constantTimeEq {
		return dataconstants.EmptyString, false
	}

	return payload, true
}

// Without a key nothing can be signed or verified, so the caller treats the
// entry as unusable rather than trusting it blindly.
func entryMAC(binding, payload string) (string, bool) {
	key := strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken))
	if key == dataconstants.EmptyString {
		return dataconstants.EmptyString, false
	}

	// The binding is folded to a fixed width first: concatenating it with the
	// payload would let a binding ending in the separator stand in for a longer
	// one, so two different pairs could share a signature.
	bound := sha256.Sum256([]byte(signatureLabel + binding))

	digest := hmac.New(sha256.New, []byte(key))
	_, _ = digest.Write(bound[:])
	_, _ = digest.Write([]byte(payload))

	return hex.EncodeToString(digest.Sum(nil)), true
}
