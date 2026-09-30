package testutil

import (
	"net/http"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/auth/internal/constants"
)

// Code that dials Redis through x-ware resolves REDIS_HOST/REDIS_PORT from env.
func RedisEnv(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	return mr
}

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
