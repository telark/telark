package oidc

import (
	"context"
	"errors"
	"testing"

	telarkconfigresource "github.com/telark/data/resources/telarkconfig"
	userresource "github.com/telark/data/resources/user"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
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
		cfg  telarkconfigresource.OIDCConfig
		want bool
	}{
		{"enabled with egress", telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, EgressAllowed: true}, true},
		{"disabled", telarkconfigresource.OIDCConfig{Enabled: false}, false},
		{"no client id", telarkconfigresource.OIDCConfig{Enabled: true}, false},
		{"no trust source", telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID}, false},
		{"offline jwk", telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, GoogleJWKJSON: "{}"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "usable", oidc.Usable(c.cfg), c.want)
		})
	}
}

// Validate rejects a config that would break login before it is stored.
func TestValidate(t *testing.T) {
	trustFile(t)
	cases := []struct {
		name    string
		cfg     telarkconfigresource.OIDCConfig
		wantErr bool
	}{
		{"disabled ok", telarkconfigresource.OIDCConfig{Enabled: false}, false},
		{"enabled valid", telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, EgressAllowed: true}, false},
		{"missing client id", telarkconfigresource.OIDCConfig{Enabled: true, EgressAllowed: true}, true},
		{"missing trust source", telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID}, true},
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
	const email = "test@example.com"
	jane := &userresource.User{ID: "u-1", Email: email}
	twin := &userresource.User{ID: "u-2", Email: "Test@Example.com"}
	other := &userresource.User{ID: "u-3", Email: "other@example.com"}
	bound := &userresource.User{ID: "u-4", Email: email,
		Identities: []*userresource.UserIdentity{{Provider: "passkey", Subject: "cred"}}}
	bootstrap := &userresource.User{ID: "u-5", Email: email, Bootstrap: true}

	cases := []struct {
		name    string
		users   []*userresource.User
		want    *userresource.User
		wantErr error
	}{
		{"none", []*userresource.User{other, nil}, nil, nil},
		{"one, case-insensitive", []*userresource.User{other, twin}, twin, nil},
		{"two", []*userresource.User{jane, twin, other}, nil, oidchandler.ErrEmailAmbiguous},
		{"already bound to another identity", []*userresource.User{bound, other}, nil, oidchandler.ErrEmailAlreadyBound},
		{"bootstrap admin, even before its passkey", []*userresource.User{bootstrap, other}, nil, oidchandler.ErrEmailAlreadyBound},
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

// SSO never grants Admin or the bootstrap marker, not even to the bootstrap email:
// that account is created and recovered only through break-glass.
func TestBuildOIDCUserIsReadOnlyForTheBootstrapEmail(t *testing.T) {
	const email = "test@example.com"
	t.Setenv(constants.EnvBootstrapAdmin, email)
	if _, err := config.LoadBootstrapConfig(); err != nil {
		t.Fatalf("LoadBootstrapConfig: %v", err)
	}
	claims := &oidc.GoogleClaims{Email: "Test@Example.com"}
	claims.Subject = "sub"

	user := oidchandler.BuildOIDCUser(claims, "test")
	testutil.Equal(t, "bootstrap", user.Bootstrap, false)
	testutil.Equal(t, "roles", len(user.RoleRefs), constants.DefaultIncrementValue)
	testutil.Equal(t, "role", *user.RoleRefs[constants.DefaultInitValue], constants.BuiltInRoleReadOnly)
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
