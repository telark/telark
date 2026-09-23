package oidc

import (
	"context"
	"testing"

	globalconfigresource "github.com/telark/data/resources/globalconfig"

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
