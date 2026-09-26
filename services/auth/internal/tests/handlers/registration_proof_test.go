package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	oidchandler "github.com/telark/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/auth/internal/handlers/passkey"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
	roledata "github.com/telark/data/resources/role"
	userresource "github.com/telark/data/resources/user"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	bootstrapEmail = "admin@example.com"
	newEmail       = "new@example.com"
)

// proofStub answers user lookups by email: the bootstrap and existing accounts
// exist, anything else is unknown; passkey lookups fail when asked to.
type proofStub struct {
	passkeyFailure bool
	created        []userresource.UserAsResource
}

func (s *proofStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(r.URL.Path, "/sessions/"):
		return envelope(http.StatusNotFound, nil)
	case strings.Contains(r.URL.Path, "passkeys"):
		if s.passkeyFailure {
			return envelope(http.StatusInternalServerError, nil)
		}
		return envelope(http.StatusOK, map[string]any{"items": []any{}})
	case strings.Contains(r.URL.Path, "findbyemail") && strings.Contains(r.URL.Path, "new"):
		if len(s.created) == constants.DefaultInitValue {
			return envelope(http.StatusNotFound, nil)
		}
		return envelope(http.StatusOK, s.created[constants.DefaultInitValue])
	case r.Method == http.MethodPost:
		var user userresource.UserAsResource
		if err := decodeInto(r, &user); err != nil {
			return nil, err
		}
		user.ID = "u-new"
		s.created = append(s.created, user)
		return envelope(http.StatusCreated, user)
	default:
		return envelope(http.StatusOK, userresource.UserAsResource{ID: "uid", Email: "a@b.com"})
	}
}

// A bare email may only open a brand-new ReadOnly account: an existing account
// needs a session or an enrollment link, a bootstrap email is reserved for the
// operator, and a passkey lookup that failed never reads as "no passkeys".
func TestRegisterStartProofRules(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmins, bootstrapEmail)
	t.Setenv(constants.EnvSelfRegistrationEnabled, "true")
	if _, err := config.LoadBootstrapConfig(); err != nil {
		t.Fatalf("LoadBootstrapConfig: %v", err)
	}
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	cases := []struct {
		name           string
		body           string
		passkeyFailure bool
		status         int
	}{
		{"bootstrap email is reserved", `{"email":"` + bootstrapEmail + `"}`, false, http.StatusForbidden},
		{"existing account without passkeys", emailBody, false, http.StatusUnauthorized},
		{"existing account, passkey lookup down", emailBody, true, http.StatusUnauthorized},
		{"unknown email self-registers", `{"email":"` + newEmail + `"}`, false, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &proofStub{passkeyFailure: c.passkeyFailure}
			stubExporter(t, stub)
			rec := httptest.NewRecorder()
			passkeyhandler.RegisterStart(rec, jsonReq(c.body))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			if c.status != http.StatusOK {
				return
			}
			if len(stub.created) != constants.DefaultIncrementValue {
				t.Fatalf("created users = %d, want 1", len(stub.created))
			}
			created := stub.created[constants.DefaultInitValue]
			if created.Bootstrap || len(created.AssignedRolesIDs) != constants.DefaultIncrementValue ||
				*created.AssignedRolesIDs[constants.DefaultInitValue] != constants.BuiltInRoleReadOnly {
				t.Fatalf("self-registered user must be ReadOnly without the bootstrap marker, got %+v", created)
			}
		})
	}
}

// The identity-provider trust is changed only by a caller who is Admin on every
// scope; Admin on settings alone is refused before the body is read.
func TestSetConfigNeedsAdminOnAll(t *testing.T) {
	cases := []struct {
		name   string
		grants xauthz.Grants
		status int
	}{
		{"settings admin only", xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeSettings: roledata.PermissionLevelAdmin}}, http.StatusForbidden},
		{"admin on all", xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeAll: roledata.PermissionLevelAdmin}}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: "uid", Grants: c.grants})
			rec := httptest.NewRecorder()
			oidchandler.SetConfig(rec, jsonReq("{").WithContext(ctx))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
		})
	}
	if authz.CallerIsAdminOnAll(context.Background()) {
		t.Fatal("no identity must not count as Admin")
	}
}

func decodeInto(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
