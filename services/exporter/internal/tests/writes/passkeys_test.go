package writes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/services/exporter/internal/constants"
	passkeyexporter "github.com/telark/telark/services/exporter/internal/exporters/auth/passkey"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ownPasskey     = "tpk-own"
	otherPasskey   = "tpk-other"
	missingPasskey = "tpk-missing"
	keyDeviceName  = "deviceName"
)

func passkeySeed(name, owner string) seed {
	md := v1alpha1.PasskeyMetadata
	return crSeed(md, object(md, name, map[string]any{"userId": owner, "credentialId": name}, nil))
}

// A deleted user's passkeys go with it; the auth sweeper only purges sessions.
func TestPurgePasskeysForUser(t *testing.T) {
	client := installFake(t, passkeySeed(ownPasskey, userID), passkeySeed(otherPasskey, otherUserID))
	if err := passkeyutils.PurgePasskeysForUser(userID); err != nil {
		t.Fatal(err)
	}
	md := v1alpha1.PasskeyMetadata
	list, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != constants.DefaultIncrementValue || list.Items[firstItem].GetName() != otherPasskey {
		t.Fatalf("remaining passkeys = %v, want only %s", list.Items, otherPasskey)
	}
}

// Another account's passkey answers exactly like an unknown one on every route
// that names a credential, and is left untouched.
func TestForeignPasskeyAnswersLikeAnUnknownOne(t *testing.T) {
	routes := []struct {
		name string
		call func(w http.ResponseWriter, credentialID string)
	}{
		{"get", func(w http.ResponseWriter, id string) { passkeyexporter.GetPasskeyByCredentialID(w, id, userID) }},
		{"patch", func(w http.ResponseWriter, id string) {
			passkeyexporter.PatchPasskeyByCredentialID(w, id, userID, map[string]any{keyDeviceName: newName})
		}},
		{"delete", func(w http.ResponseWriter, id string) {
			passkeyexporter.DeletePasskeyByCredentialID(w, id, userID, true)
		}},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			client := installFake(t, passkeySeed(otherPasskey, otherUserID))
			unknown, foreign := httptest.NewRecorder(), httptest.NewRecorder()
			route.call(unknown, missingPasskey)
			route.call(foreign, otherPasskey)
			if foreign.Code != http.StatusNotFound || foreign.Code != unknown.Code || foreign.Body.String() != unknown.Body.String() {
				t.Fatalf("foreign = %d %s, unknown = %d %s", foreign.Code, foreign.Body, unknown.Code, unknown.Body)
			}
			if got := writes(t, client); len(got) != constants.DefaultInitValue {
				t.Fatalf("writes = %v, want none", got)
			}
			stored(t, client, v1alpha1.PasskeyMetadata, otherPasskey)
		})
	}
}
