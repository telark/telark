package oidc

import (
	"context"
	"errors"
	"testing"

	globalconfigresource "github.com/telark/data/resources/globalconfig"
	userresource "github.com/telark/data/resources/user"

	oidchandler "github.com/telark/auth/internal/handlers/oidc"
	"github.com/telark/auth/internal/helpers/oidc"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	"github.com/telark/auth/internal/tests/testutil"
)

const clientID = "id"

// Usable answers "can we run SSO with this config" — enabled, a client id, and a
// trust source (live egress or a pasted key set).
func TestUsable(t *testing.T) {
	cases := []struct {
		name string
		cfg  globalconfigresource.OIDCConfig
		want bool
	}{
		{"enabled with egress", globalconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, EgressAllowed: true}, true},
		{"disabled", globalconfigresource.OIDCConfig{Enabled: false}, false},
		{"no client id", globalconfigresource.OIDCConfig{Enabled: true}, false},
		{"no trust source", globalconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID}, false},
		{"offline jwk", globalconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, GoogleJWKJSON: "{}"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "usable", oidc.Usable(c.cfg), c.want)
		})
	}
}

// Validate rejects a config that would break login before it is stored.
func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     globalconfigresource.OIDCConfig
		wantErr bool
	}{
		{"disabled ok", globalconfigresource.OIDCConfig{Enabled: false}, false},
		{"enabled valid", globalconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, EgressAllowed: true}, false},
		{"missing client id", globalconfigresource.OIDCConfig{Enabled: true, EgressAllowed: true}, true},
		{"missing trust source", globalconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "err", oidc.Validate(c.cfg) != nil, c.wantErr)
		})
	}
}

// A Google identity is attached by email only when exactly one user carries it;
// two candidates refuse rather than bind the identity to whichever came first.
func TestUserForEmail(t *testing.T) {
	const email = "jane.doe@example.com"
	jane := &userresource.UserAsResource{ID: "u-1", Email: email}
	twin := &userresource.UserAsResource{ID: "u-2", Email: "Jane.Doe@example.com"}
	other := &userresource.UserAsResource{ID: "u-3", Email: "other@example.com"}

	cases := []struct {
		name    string
		users   []*userresource.UserAsResource
		want    *userresource.UserAsResource
		wantErr error
	}{
		{"none", []*userresource.UserAsResource{other, nil}, nil, nil},
		{"one, case-insensitive", []*userresource.UserAsResource{other, twin}, twin, nil},
		{"two", []*userresource.UserAsResource{jane, twin, other}, nil, oidchandler.ErrEmailAmbiguous},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := oidchandler.UserForEmail(c.users, email)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			testutil.Equal(t, "user", got, c.want)
		})
	}
}

// A nonce is single-use: generated + stored in Redis, verified once, then gone.
func TestNonceLifecycle(t *testing.T) {
	testutil.RedisEnv(t)
	redishelper.NewRedisClientWithRetry(context.Background()) // bind cached client to embedded redis

	nonce, err := oidc.GenerateAndStoreNonce()
	if err != nil || nonce == "" {
		t.Fatalf("GenerateAndStoreNonce = (%q, %v)", nonce, err)
	}
	if err := oidc.VerifyAndConsumeNonce(nonce); err != nil {
		t.Fatalf("first verify = %v, want nil", err)
	}
	if oidc.VerifyAndConsumeNonce(nonce) == nil {
		t.Fatal("second verify should fail — nonce is single-use")
	}
	if oidc.VerifyAndConsumeNonce("") == nil {
		t.Fatal("empty nonce should be rejected")
	}
}
