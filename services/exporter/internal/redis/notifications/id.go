package notifications

import (
	"crypto/rand"
	"encoding/hex"
)

const idByteLen = 16

func newID() string {
	b := make([]byte, idByteLen)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
