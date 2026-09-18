package webauthn

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
	"github.com/telark/auth/internal/tests/testutil"
)

// With RP_ID / RP_ORIGIN empty the relying party follows the request; a pinned
// origin list keeps today's behaviour and refuses any other Origin with the
// 400-style error. The same (rpID, origin) pair always yields the same instance.
func TestGetWebAuthnForResolvesRelyingPartyPerRequest(t *testing.T) {
	t.Cleanup(func() { _ = webauthnhelper.InitWebAuthn(&pinnedRelyingParty) })

	follow := config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}
	pinnedID := config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60, RPID: "example.com"}
	pinnedList := config.WebAuthnConfig{
		RPName: "Test", ChallengeTimeout: 60,
		RPID: "example.com", RPOrigin: "https://a.example.com, https://b.example.com",
	}

	cases := []struct {
		name          string
		cfg           config.WebAuthnConfig
		host          string
		forwardedHost string
		origin        string
		wantRPID      string
		wantOrigins   []string
		wantErr       bool
	}{
		{
			name: "empty config follows host and origin", cfg: follow,
			host: "app.example.com", origin: "https://app.example.com",
			wantRPID: "app.example.com", wantOrigins: []string{"https://app.example.com"},
		},
		{
			name: "empty config prefers X-Forwarded-Host and strips the port", cfg: follow,
			host: "ui:8080", forwardedHost: "app.example.com:3000", origin: "https://app.example.com:3000",
			wantRPID: "app.example.com", wantOrigins: []string{"https://app.example.com:3000"},
		},
		{
			name: "empty config falls back to scheme and host without Origin", cfg: follow,
			host:     "app.example.com",
			wantRPID: "app.example.com", wantOrigins: []string{"http://app.example.com"},
		},
		{
			name: "empty config rejects an origin on another host", cfg: follow,
			host: "app.example.com", origin: "https://evil.example.org", wantErr: true,
		},
		{
			name: "pinned id accepts a subdomain origin", cfg: pinnedID,
			host: "app.example.com", origin: "https://app.example.com",
			wantRPID: "example.com", wantOrigins: []string{"https://app.example.com"},
		},
		{
			name: "pinned list accepts a listed origin", cfg: pinnedList,
			host: "app.example.com", origin: "https://b.example.com",
			wantRPID: "example.com", wantOrigins: []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name: "pinned list rejects a foreign origin", cfg: pinnedList,
			host: "app.example.com", origin: "https://evil.example.org", wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := webauthnhelper.InitWebAuthn(&c.cfg); err != nil {
				t.Fatalf("InitWebAuthn = %v", err)
			}
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.Host = c.host
			if c.forwardedHost != "" {
				r.Header.Set(constants.HeaderForwardedHost, c.forwardedHost)
			}
			if c.origin != "" {
				r.Header.Set(constants.HeaderOrigin, c.origin)
			}

			wa, err := webauthnhelper.GetWebAuthnFor(r)
			if c.wantErr {
				if !shared.IsError(err, constants.ErrOriginNotAllowed) {
					t.Fatalf("GetWebAuthnFor err = %v, want %q", err, constants.ErrOriginNotAllowed)
				}
				testutil.Equal(t, "status", shared.GetStatusCodeForWebAuthnError(err, http.StatusUnauthorized), http.StatusBadRequest)
				return
			}
			if err != nil {
				t.Fatalf("GetWebAuthnFor = %v", err)
			}
			testutil.Equal(t, "rpID", wa.Config.RPID, c.wantRPID)
			if !slices.Equal(wa.Config.RPOrigins, c.wantOrigins) {
				t.Fatalf("origins = %v, want %v", wa.Config.RPOrigins, c.wantOrigins)
			}

			again, err := webauthnhelper.GetWebAuthnFor(r)
			if err != nil || again != wa {
				t.Fatalf("second resolution = (%p, %v), want the cached %p", again, err, wa)
			}
		})
	}
}
