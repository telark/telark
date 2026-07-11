package shared

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
	"github.com/telark/auth/internal/constants"
)

func GenerateSessionToken() (string, error) {
	return uuid.New().String(), nil
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
