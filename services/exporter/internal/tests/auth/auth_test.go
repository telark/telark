package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/constants"
	passkeyutil "github.com/telark/exporter/internal/utils/auth/passkey"
	sessionutil "github.com/telark/exporter/internal/utils/auth/session"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	testUserID       = "u1"
	testCredentialID = "c1"
	testSessionToken = "tok"
)

func TestExtractPasskeySpecValidation(t *testing.T) {
	// A missing required field fails before any uniqueness lookup.
	if _, _, err := passkeyutil.ExtractPasskeySpec(map[string]any{"publicKey": "k"}, testUserID); err == nil {
		t.Error("passkey without credentialId accepted")
	}
	// All required fields present but an invalid device type is rejected.
	body := map[string]any{
		"credentialId": testCredentialID, "publicKey": "k", "deviceName": "phone", "deviceType": "hologram",
	}
	if _, _, err := passkeyutil.ExtractPasskeySpec(body, testUserID); err == nil {
		t.Error("invalid device type accepted")
	}
}

func TestExtractPatchFields(t *testing.T) {
	ok, err := passkeyutil.ExtractPatchFields(map[string]any{"deviceName": "n", "lastUsedTimestamp": "t"})
	if err != nil || ok["deviceName"] != "n" {
		t.Fatalf("valid patch = %v, err %v", ok, err)
	}
	if _, err := passkeyutil.ExtractPatchFields(map[string]any{"deviceName": "n", "credentialId": "x"}); err == nil {
		t.Error("disallowed patch field accepted")
	}
}

func TestExtractPasskeyRequestParams(t *testing.T) {
	missing := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	if _, _, _, ok := passkeyutil.ExtractPasskeyRequestParams(httptest.NewRecorder(), missing); ok {
		t.Error("request without user header or credential id accepted")
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"deviceName":"n"}`))
	r.Header.Set(constants.HeaderUserID, testUserID)
	r = mux.SetURLVars(r, map[string]string{constants.CredentialIDParam: testCredentialID})
	userID, credID, body, ok := passkeyutil.ExtractPasskeyRequestParams(httptest.NewRecorder(), r)
	if !ok || userID != testUserID || credID != testCredentialID || body == nil {
		t.Errorf("valid request rejected: ok=%v userID=%q credID=%q", ok, userID, credID)
	}
}

func TestExtractSessionSpecValidation(t *testing.T) {
	if _, _, err := sessionutil.ExtractSessionSpec(map[string]any{}, testUserID); err == nil {
		t.Error("session without token accepted")
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	body := map[string]any{"sessionToken": testSessionToken, "expiresTimestamp": past}
	if _, _, err := sessionutil.ExtractSessionSpec(body, testUserID); err == nil {
		t.Error("already-expired session accepted")
	}
}

func TestUnstructuredToSession(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"sessionToken": testSessionToken}}}
	s, err := sessionutil.UnstructuredToSession(res)
	if err != nil || s.SessionToken != testSessionToken {
		t.Fatalf("UnstructuredToSession = %+v, err %v", s, err)
	}
	if _, err := sessionutil.UnstructuredToSession(&unstructured.Unstructured{Object: map[string]any{}}); err == nil {
		t.Error("missing spec accepted")
	}
}
