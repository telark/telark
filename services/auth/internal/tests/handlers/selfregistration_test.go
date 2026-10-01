package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	confighandler "github.com/telark/telark/services/auth/internal/handlers/config"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	// Pinned, not read from constants: the docs promise a flip applies within about five seconds.
	configCacheTTL  = 5 * time.Second
	cacheGrace      = time.Second
	cachePoll       = 50 * time.Millisecond
	loginPageLoads  = 20
	bootstrapCallID = "u-bootstrap"
	adminCallID     = "u-admin-all"
	selfRegOnBody   = `{"enabled":true}`
	malformedJSON   = "{"
)

func selfRegistrationOn() telarkconfigresource.TelarkConfig {
	return telarkconfigresource.TelarkConfig{SelfRegistration: telarkconfigresource.SelfRegistrationConfig{Enabled: true}}
}

// The flag is cached for a few seconds, so a test that needs a value waits for the
// cache to catch up with its stub instead of assuming it already has.
func awaitSelfRegistration(t *testing.T, want bool) {
	t.Helper()
	deadline := time.Now().Add(configCacheTTL + cacheGrace)
	for config.IsSelfRegistrationEnabled() != want {
		if time.Now().After(deadline) {
			t.Fatalf("self-registration still %v one cache TTL after the config changed", !want)
		}
		time.Sleep(cachePoll)
	}
}

func publicSelfRegistration(t *testing.T) bool {
	t.Helper()
	rec := httptest.NewRecorder()
	confighandler.GetConfig(rec, httptest.NewRequest(http.MethodGet, testPath, nil))
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode public config: %v", err)
	}
	enabled, isBool := body.Data[constants.JSONKeySelfRegEnabled].(bool)
	if !isBool {
		t.Fatalf("%s = %v, want a boolean", constants.JSONKeySelfRegEnabled, body.Data[constants.JSONKeySelfRegEnabled])
	}
	return enabled
}

// The login page and the passkey path follow a flip within the cache TTL, a failed read keeps
// the last value, and the exporter is asked at most once per TTL however often the page loads.
func TestSelfRegistrationFollowsTheTelarkConfig(t *testing.T) {
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	fake := &testutil.FakeExporter{Config: selfRegistrationOn()}
	testutil.StubBackend(t, fake)
	awaitSelfRegistration(t, true)
	testutil.Equal(t, "public config while on", publicSelfRegistration(t), true)
	testutil.Equal(t, "self-registration start while on", startSelfRegistration(), http.StatusOK)

	fake.SetDown(true)
	failedRead := fake.ConfigReads() + constants.DefaultIncrementValue
	deadline := time.Now().Add(configCacheTTL + cacheGrace)
	for fake.ConfigReads() < failedRead && time.Now().Before(deadline) {
		testutil.Equal(t, "public config while the exporter fails", publicSelfRegistration(t), true)
		time.Sleep(cachePoll)
	}
	for range loginPageLoads {
		testutil.Equal(t, "public config after a failed read", publicSelfRegistration(t), true)
	}
	testutil.Equal(t, "exporter reads in one TTL", fake.ConfigReads(), failedRead)

	fake.SetConfig(telarkconfigresource.TelarkConfig{})
	fake.SetDown(false)
	awaitSelfRegistration(t, false)
	testutil.Equal(t, "public config once off", publicSelfRegistration(t), false)
	testutil.Equal(t, "self-registration start once off", startSelfRegistration(), http.StatusForbidden)
}

func startSelfRegistration() int {
	rec := httptest.NewRecorder()
	passkeyhandler.RegisterStart(rec, jsonReq(`{"email":"`+newEmail+`"}`))
	return rec.Code
}

func signInSettingsReq(caller xauthz.Identity, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, router.Pattern(autheps.SelfRegistration), strings.NewReader(body))
	return r.WithContext(xauthz.WithIdentity(r.Context(), caller))
}

// Who may sign in, and how, is changed only by the bootstrap account, whatever grants
// a session holds; a caller record that cannot be read is an outage, not a refusal.
func TestSignInSettingsAreBootstrapOnly(t *testing.T) {
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		body    string
		caller  string
		down    bool
		status  int
	}{
		{"sso by an Admin on ALL", oidchandler.SetConfig, malformedJSON, adminCallID, false, http.StatusForbidden},
		{"sso by the bootstrap account reaches the body", oidchandler.SetConfig, malformedJSON, bootstrapCallID, false, http.StatusBadRequest},
		{"sso with the caller unreadable", oidchandler.SetConfig, malformedJSON, bootstrapCallID, true, http.StatusServiceUnavailable},
		{"self-registration by an Admin on ALL", routed, selfRegOnBody, adminCallID, false, http.StatusForbidden},
		{"self-registration by the bootstrap account", routed, selfRegOnBody, bootstrapCallID, false, http.StatusOK},
		{"self-registration with the caller unreadable", routed, selfRegOnBody, bootstrapCallID, true, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &testutil.FakeExporter{Users: map[string]*userresource.User{
				bootstrapCallID: {ID: bootstrapCallID, Bootstrap: true},
				adminCallID:     {ID: adminCallID},
			}}
			testutil.StubBackend(t, fake)
			fake.SetDown(c.down)
			rec := httptest.NewRecorder()
			caller := adminOnAll()
			caller.UserID = c.caller
			c.handler(rec, signInSettingsReq(caller, c.body))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			if c.status == http.StatusOK && !fake.SavedConfig().SelfRegistration.Enabled {
				t.Fatal("the bootstrap account's change was not saved")
			}
		})
	}
}
