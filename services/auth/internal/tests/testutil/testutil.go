package testutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/fxamacker/cbor/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/auth/internal/constants"
)

// COSE key map labels and values for an EC2 key on P-256 (RFC 9053).
const (
	coseLabelKty  = 1
	coseLabelAlg  = 3
	coseLabelCrv  = -1
	coseLabelX    = -2
	coseLabelY    = -3
	coseKtyEC2    = 2
	coseCrvP256   = 1
	p256CoordLen  = 32
	uncompressedX = 1
)

func RedisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// Handlers called directly skip the router, so the path variable it would set is set here.
func WithCredentialID(r *http.Request, credentialID string) *http.Request {
	return mux.SetURLVars(r, map[string]string{constants.CredentialIDPathParam: credentialID})
}

// What Redis holds for a WebAuthn challenge, in its keys and values: the SHA-256 of its bytes, never the challenge.
func ChallengeDigest(t *testing.T, challengeB64 string) string {
	t.Helper()
	decoded, err := base64.RawURLEncoding.DecodeString(challengeB64)
	if err != nil {
		t.Fatalf("challenge %q: %v", challengeB64, err)
	}
	sum := sha256.Sum256(decoded)
	return hex.EncodeToString(sum[:])
}

func NewP256Key(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	return priv
}

// A passkey public key as an authenticator encodes it; alg can name an algorithm the
// relying party never offered.
func COSEKeyEC2(t *testing.T, pub *ecdsa.PublicKey, alg int) []byte {
	t.Helper()
	point, err := pub.Bytes() // 0x04 || X || Y
	if err != nil {
		t.Fatalf("public key bytes: %v", err)
	}
	key, err := cbor.Marshal(map[int]any{
		coseLabelKty: coseKtyEC2, coseLabelAlg: alg, coseLabelCrv: coseCrvP256,
		coseLabelX: point[uncompressedX : uncompressedX+p256CoordLen], coseLabelY: point[uncompressedX+p256CoordLen:],
	})
	if err != nil {
		t.Fatalf("cose key: %v", err)
	}
	return key
}
