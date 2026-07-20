package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/telark/exporter/internal/constants"
	passkeyutil "github.com/telark/exporter/internal/utils/auth/passkey"
	sessionutil "github.com/telark/exporter/internal/utils/auth/session"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractPasskeySpecValidation(t *testing.T) {
	// A missing required field fails before any uniqueness lookup.
	if _, _, err := passkeyutil.ExtractPasskeySpec(map[string]any{"publicKey": "k"}, "u1"); err == nil {
		t.Error("passkey without credentialId accepted")
	}
	// All required fields present but an invalid device type is rejected.
	body := map[string]any{
		"credentialId": "c1", "publicKey": "k", "deviceName": "phone", "deviceType": "hologram",
	}
	if _, _, err := passkeyutil.ExtractPasskeySpec(body, "u1"); err == nil {
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

func TestUnstructuredToPasskey(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"credentialId": "c1"}}}
	pk, err := passkeyutil.UnstructuredToPasskey(res)
	if err != nil || pk.CredentialID != "c1" {
		t.Fatalf("UnstructuredToPasskey = %+v, err %v", pk, err)
	}
	if _, err := passkeyutil.UnstructuredToPasskey(&unstructured.Unstructured{Object: map[string]any{}}); err == nil {
		t.Error("missing spec accepted")
	}
}

func TestExtractPasskeyRequestParams(t *testing.T) {
	missing := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	if _, _, _, ok := passkeyutil.ExtractPasskeyRequestParams(httptest.NewRecorder(), missing); ok {
		t.Error("request without headers accepted")
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"deviceName":"n"}`))
	r.Header.Set(constants.HeaderUserID, "u1")
	r.Header.Set(constants.HeaderCredentialID, "c1")
	userID, credID, body, ok := passkeyutil.ExtractPasskeyRequestParams(httptest.NewRecorder(), r)
	if !ok || userID != "u1" || credID != "c1" || body == nil {
		t.Errorf("valid request rejected: ok=%v userID=%q credID=%q", ok, userID, credID)
	}
}

func TestExtractSessionSpecValidation(t *testing.T) {
	if _, _, err := sessionutil.ExtractSessionSpec(map[string]any{}, "u1"); err == nil {
		t.Error("session without token accepted")
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	body := map[string]any{"sessionToken": "tok", "expiresTimestamp": past}
	if _, _, err := sessionutil.ExtractSessionSpec(body, "u1"); err == nil {
		t.Error("already-expired session accepted")
	}
}

func TestUnstructuredToSession(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"sessionToken": "tok"}}}
	s, err := sessionutil.UnstructuredToSession(res)
	if err != nil || s.SessionToken != "tok" {
		t.Fatalf("UnstructuredToSession = %+v, err %v", s, err)
	}
	if _, err := sessionutil.UnstructuredToSession(&unstructured.Unstructured{Object: map[string]any{}}); err == nil {
		t.Error("missing spec accepted")
	}
}
