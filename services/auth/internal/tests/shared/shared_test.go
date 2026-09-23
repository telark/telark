package shared

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
)

const randomBytesLen = 16

func TestValidateUsername(t *testing.T) {
	if err := shared.ValidateUsername("alice"); err != nil {
		t.Fatalf("valid username rejected: %v", err)
	}
	if shared.ValidateUsername(constants.EmptyString) == nil {
		t.Fatal("empty username accepted")
	}
}

// Email validation must reject empties and malformed addresses — the login flow
// keys accounts on a real address.
func TestValidateEmail(t *testing.T) {
	cases := []struct {
		email   string
		wantErr bool
	}{
		{"a@b.com", false},
		{constants.EmptyString, true},
		{"not-an-email", true},
	}
	for _, c := range cases {
		if (shared.ValidateEmail(c.email) != nil) != c.wantErr {
			t.Fatalf("ValidateEmail(%q) err mismatch, want %v", c.email, c.wantErr)
		}
	}
}

// Session tokens are unique per call; the random helpers round-trip cleanly
// through base64url.
func TestTokenHelpers(t *testing.T) {
	a, _ := shared.GenerateSessionToken()
	b, _ := shared.GenerateSessionToken()
	if a == constants.EmptyString || a == b {
		t.Fatalf("session tokens not unique: %q %q", a, b)
	}

	raw, err := shared.GenerateRandomBytes(randomBytesLen)
	if err != nil || len(raw) != randomBytesLen {
		t.Fatalf("GenerateRandomBytes = %v (len %d)", err, len(raw))
	}
	enc := shared.Base64URLEncode(raw)
	dec, err := shared.Base64URLDecode(enc)
	if err != nil || !bytes.Equal(dec, raw) {
		t.Fatalf("base64url round-trip failed: %v", err)
	}
	if _, err := shared.Base64URLDecode("!!!not-base64!!!"); err == nil {
		t.Fatal("invalid base64url decoded without error")
	}
}

func TestIsError(t *testing.T) {
	target := constants.ErrMissingCredentialID
	if shared.IsError(nil, target) {
		t.Fatal("nil error matched a target")
	}
	if !shared.IsError(errString(string(target)), target) {
		t.Fatal("matching error not detected")
	}
	if shared.IsError(errString("other"), target) {
		t.Fatal("non-matching error matched")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// The JSON response helpers set the content type + status and encode a
// success/error envelope the dashboard expects.
func TestResponseHelpers(t *testing.T) {
	t.Run("success with data", func(t *testing.T) {
		w := httptest.NewRecorder()
		shared.SendSuccessResponse(w, "ok", map[string]any{"k": "v"})
		assertJSON(t, w, http.StatusOK, constants.JSONKeySuccess, true)
	})
	t.Run("success without data", func(t *testing.T) {
		w := httptest.NewRecorder()
		shared.SendSuccessResponse(w, "ok", nil)
		assertJSON(t, w, http.StatusOK, constants.JSONKeyMessage, "ok")
	})
	t.Run("error", func(t *testing.T) {
		w := httptest.NewRecorder()
		shared.SendErrorResponse(w, http.StatusBadRequest, errString("bad"))
		assertJSON(t, w, http.StatusBadRequest, constants.JSONKeyError, true)
	})
	t.Run("handle error logs and responds", func(t *testing.T) {
		w := httptest.NewRecorder()
		shared.HandleError(w, errString("boom"), http.StatusUnauthorized, "log this")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

func TestDecodeRequestBody(t *testing.T) {
	var out map[string]string
	ok := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":"b"}`))
	if err := shared.DecodeRequestBody(ok, &out); err != nil || out["a"] != "b" {
		t.Fatalf("decode valid body = %v, out=%v", err, out)
	}
	bad := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{bad`))
	if shared.DecodeRequestBody(bad, &out) == nil {
		t.Fatal("malformed body decoded without error")
	}
}

// Auth errors map to 400 only for a missing credential id; everything else is a
// 401 so probing does not leak which step failed.
func TestGetStatusCodeForAuthError(t *testing.T) {
	if got := shared.GetStatusCodeForAuthError(errString(string(constants.ErrMissingCredentialID))); got != http.StatusBadRequest {
		t.Fatalf("missing credential id = %d, want 400", got)
	}
	if got := shared.GetStatusCodeForAuthError(errString("nope")); got != http.StatusUnauthorized {
		t.Fatalf("generic auth error = %d, want 401", got)
	}
}

func assertJSON(t *testing.T, w *httptest.ResponseRecorder, wantCode int, key string, wantVal any) {
	t.Helper()
	if w.Code != wantCode {
		t.Fatalf("status = %d, want %d", w.Code, wantCode)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body[key] != wantVal {
		t.Fatalf("body[%q] = %v, want %v", key, body[key], wantVal)
	}
}
