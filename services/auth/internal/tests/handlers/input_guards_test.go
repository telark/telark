package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/auth/internal/constants"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	displayNameEmail = `{"email":"Release Lead <test@example.com>"}`
	metadataCut      = 300
	invisibleRune    = 0x200B
)

// The browser fills the device metadata, so each value is cut to 256 characters and cleaned, never refused; a field
// the browser left out stays out.
func TestSignInTruncatesDeviceMetadata(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStateCompleted)
	testutil.StubBackend(t, fake)
	atLimit := strings.Repeat("b", constants.DeviceMetadataMaxLen)
	rec := httptest.NewRecorder()
	authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t), map[string]any{
		"email":     " " + bootstrapEmail + " ",
		"browser":   atLimit[1:],
		"device":    atLimit,
		"os":        atLimit + "b",
		"userAgent": "  Mozilla" + string(rune(invisibleRune)) + "/5.0 " + strings.Repeat("u", metadataCut),
	})))
	testutil.Equal(t, "status", rec.Code, http.StatusOK)
	sessions := fake.Sessions()
	testutil.Equal(t, "sessions", len(sessions), constants.DefaultIncrementValue)
	meta := sessions[constants.DefaultInitValue].DeviceMetadata
	testutil.Equal(t, "browser under the limit", *meta.Browser, atLimit[1:])
	testutil.Equal(t, "device at the limit", *meta.Device, atLimit)
	testutil.Equal(t, "os over the limit", *meta.OS, atLimit)
	userAgent := "Mozilla/5.0 " + strings.Repeat("u", metadataCut)
	testutil.Equal(t, "user agent cleaned and cut", *meta.UserAgent, userAgent[:constants.DeviceMetadataMaxLen])
	testutil.Equal(t, "location left out", meta.Location == nil, true)
}

// Only a bare address signs in or registers: the display-name forms net/mail accepts answer 400 before any lookup.
func TestDisplayNameEmailIsRefused(t *testing.T) {
	for name, handler := range map[string]handlerFunc{
		"login start":    authhandler.LoginStart,
		"login finish":   authhandler.LoginFinish,
		"register start": passkeyhandler.RegisterStart,
	} {
		t.Run(name, func(t *testing.T) {
			fake := &testutil.FakeExporter{Config: selfRegistrationOn()}
			testutil.StubBackend(t, fake)
			rec := httptest.NewRecorder()
			handler(rec, jsonReq(displayNameEmail))
			testutil.Equal(t, "status", rec.Code, http.StatusBadRequest)
			testutil.Equal(t, "names the email", strings.Contains(rec.Body.String(), string(constants.ErrInvalidEmail)), true)
		})
	}
}
