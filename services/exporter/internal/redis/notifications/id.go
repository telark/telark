package notifications

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/telark/exporter/internal/constants"
)

const idByteLen = 16

func newID() string {
	b := make([]byte, idByteLen)
	if _, err := rand.Read(b); err != nil {
		return constants.EmptyString
	}
	return hex.EncodeToString(b)
}
