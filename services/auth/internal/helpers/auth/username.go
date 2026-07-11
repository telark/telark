package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/telark/auth/internal/constants"
)

var usernameClean = regexp.MustCompile(constants.UsernameInvalidChars)

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
