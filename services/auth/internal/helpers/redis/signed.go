package redis

import (
	"errors"
	"strings"
	"time"

	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/constants"
)

// Redis is untrusted, so an entry that decides who registers a passkey is signed for its key and carries
// its expiry: a planted, edited, moved or replayed value then reads like no entry at all.
func SignEntry(key, value string, ttl time.Duration) (string, error) {
	expiresAt := time.Now().UTC().Add(ttl).Format(constants.TimeFormatRFC3339)
	entry, ok := xauthz.SignRedisEntry(key, expiresAt+constants.DotSeparator+value)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrRedisEntryUnsigned))
	}
	return entry, nil
}

func VerifyEntry(key, entry string) (string, bool) {
	payload, ok := xauthz.VerifyRedisEntry(key, entry)
	if !ok {
		return constants.EmptyString, false
	}
	expiry, value, found := strings.Cut(payload, constants.DotSeparator)
	expiresAt, err := time.Parse(constants.TimeFormatRFC3339, expiry)
	if !found || err != nil || !time.Now().Before(expiresAt) {
		return constants.EmptyString, false
	}
	return value, true
}
