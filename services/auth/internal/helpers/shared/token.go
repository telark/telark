package shared

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/telark/auth/internal/constants"
)

func GenerateSessionToken() (string, error) {
	raw, err := GenerateRandomBytes(constants.SessionTokenBytes)
	if err != nil {
		return constants.EmptyString, err
	}
	return Base64URLEncode(raw), nil
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGenerateRandomBytes), err)
	}
	return b, nil
}

func Base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func Base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
