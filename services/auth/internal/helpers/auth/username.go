package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/telark/telark/internal/data/text"
	"github.com/telark/telark/services/auth/internal/constants"
)

var usernameClean = regexp.MustCompile(constants.UsernameInvalidChars)

// Placeholder name for provisioning when the provider supplies none:
// "test@example.com" -> "test".
func BuildFullnameFromEmail(email string) string {
	local := strings.SplitN(email, "@", constants.EmailSplitParts)[constants.DefaultInitValue]
	if name := joinWords(local, constants.EmptyString); name != constants.EmptyString {
		return name
	}
	return constants.DefaultFullname
}

// A provider's name fitted to the user schema (letters, marks, space and ' . -, at most 100): a sign-in that
// already verified is never refused over its derived name.
func BuildFullname(name, email string) string {
	if fitted := joinWords(name, constants.FullnameExtraChars); fitted != constants.EmptyString {
		return fitted
	}
	return BuildFullnameFromEmail(email)
}

func joinWords(raw, extra string) string {
	words := strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsMark(r) && !strings.ContainsRune(extra, r)
	})
	return text.Truncate(strings.Join(words, constants.SpaceSeparator), constants.FullnameMaxLen)
}

func BuildUsername(email string) (string, error) {
	local := strings.SplitN(email, "@", constants.EmailSplitParts)[constants.DefaultInitValue]
	clean := usernameClean.ReplaceAllString(local, constants.UnderscoreSeparator)
	if len(clean) > constants.UsernameMaxLocalLen {
		clean = clean[:constants.UsernameMaxLocalLen]
	}
	b := make([]byte, constants.UsernameRandomBytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedGenerateRandomBytes), err)
	}
	username := clean + constants.UnderscoreSeparator + hex.EncodeToString(b)
	for len(username) < constants.UsernameMinLen {
		username += constants.UnderscoreSeparator
	}
	return username, nil
}
