package ids

import (
	"crypto/rand"
	"fmt"

	"github.com/telark/discovery/constants"
)

const (
	prefix      = "pp-"
	firstChunk  = 3
	middleChunk = 4
	lastChunk   = 4

	alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// GeneratePlanID returns a fresh plan identifier matching the CRD's expected pattern
// `pp-<3>-<4>-<4>` so that the exporter accepts the resource on create.
func GeneratePlanID() (string, error) {
	first, err := randomChunk(firstChunk)
	if err != nil {
		return constants.EmptyString, err
	}
	mid, err := randomChunk(middleChunk)
	if err != nil {
		return constants.EmptyString, err
	}
	last, err := randomChunk(lastChunk)
	if err != nil {
		return constants.EmptyString, err
	}
	return fmt.Sprintf("%s%s-%s-%s", prefix, first, mid, last), nil
}

func randomChunk(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return constants.EmptyString, err
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
