package serialization

import (
	"encoding/json"
	"testing"

	authdata "github.com/telark/data/auth"
	"github.com/telark/data/resources/globalconfig"
)

const (
	sampleToken  = "st-9f3c1d7b"
	sampleAPIKey = "sk-test-1234"
	sampleUserID = "user-1"

	fieldSessionToken = "sessionToken"
	fieldUserID       = "userId"
)

// Mirrors the exporter's StructToSpecMap: the struct is marshaled and the
// resulting map becomes the CR spec.
func specMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return spec
}

func TestClearedSessionTokenIsNotWrittenToTheSpec(t *testing.T) {
	spec := specMap(t, authdata.UserSession{UserID: sampleUserID})

	if _, found := spec[fieldSessionToken]; found {
		t.Errorf("spec carries %q, a key the UserSession CRD no longer declares: %v",
			fieldSessionToken, spec)
	}
	if spec[fieldUserID] != sampleUserID {
		t.Errorf("spec lost userId: %v", spec)
	}
}

func TestSessionTokenStillTravelsOnCreate(t *testing.T) {
	spec := specMap(t, authdata.UserSession{UserID: sampleUserID, SessionToken: sampleToken})

	if spec[fieldSessionToken] != sampleToken {
		t.Errorf("create payload dropped the session token: %v", spec)
	}
}

func TestSeededGlobalConfigCarriesNoAPIKey(t *testing.T) {
	spec := specMap(t, globalconfig.DefaultGlobalConfig())

	ai, found := spec[globalconfig.FieldAI].(map[string]any)
	if !found {
		t.Fatalf("spec has no %q section: %v", globalconfig.FieldAI, spec)
	}
	if _, found := ai[globalconfig.FieldAPIKey]; found {
		t.Errorf("seeded spec carries %q, a key the GlobalConfig CRD no longer declares: %v",
			globalconfig.FieldAPIKey, ai)
	}
}

func TestAPIKeyStillSerializesWhenSet(t *testing.T) {
	cfg := globalconfig.DefaultGlobalConfig()
	cfg.AI.APIKey = sampleAPIKey
	spec := specMap(t, cfg)

	ai, found := spec[globalconfig.FieldAI].(map[string]any)
	if !found {
		t.Fatalf("spec has no %q section: %v", globalconfig.FieldAI, spec)
	}
	if ai[globalconfig.FieldAPIKey] != sampleAPIKey {
		t.Errorf("a set provider key must still reach the response: %v", ai)
	}
}
