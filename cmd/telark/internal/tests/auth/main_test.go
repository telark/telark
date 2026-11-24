package auth

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"golang.org/x/crypto/nacl/box"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	namespace      = "telark"
	contextName    = "test"
	adminEmail     = "test@example.com"
	requests       = "authrequests"
	requestName    = "auth-request-00001"
	authCommand    = "auth"
	enrollCommand  = "enroll"
	statusCommand  = "status"
	emailFlag      = "--email"
	kubeconfigFlag = "--kubeconfig"
	outputFlag     = "-o"
	missingConfig  = "/nonexistent/kubeconfig"
	kubeconfigEnv  = "KUBECONFIG"
	codeLabel      = "code"
	stdoutLabel    = "stdout"
	stderrLabel    = "stderr"
	keySpec        = "spec"
	keyStatus      = "status"
	keyMetadata    = "metadata"
	keyPhase       = "phase"
	keyReason      = "reason"
	keyEmail       = "email"
	keyRequestedBy = "requestedBy"
	keyRecipient   = "recipientKey"
	keyGenerate    = "generateName"
	keySealedLink  = "sealedLink"
	keyLinkExpires = "linkExpiresAt"
	keyApproval    = "approvalUrl"
	keyCompletes   = "completesAt"
	keyBootstrap   = "bootstrap"
	keyPinnedAt    = "pinnedAt"
	keyRequests    = "requests"
	keyID          = "id"
	keyKind        = "kind"
	keyTarget      = "targetEmail"
	keyState       = "state"
	keyOpenedAt    = "openedAt"
	telarkGroup    = "telark.io/v1alpha1"
	configKind     = "TelarkConfig"
	configName     = "default"
	authRequestsAt = "authrequests.telark.io"
	keyBytes       = 32
)

func kubeconfig(t *testing.T, fake *testutil.FakeCluster) string {
	t.Helper()
	return testutil.Kubeconfig(t, contextName, map[string]string{contextName: fake.URL})
}

func enroll(t *testing.T, fake *testutil.FakeCluster) (<-chan testutil.Result, *testutil.Stream) {
	t.Helper()
	stream := &testutil.Stream{}
	return testutil.Start(t, stream, authCommand, enrollCommand, emailFlag, adminEmail, kubeconfigFlag, kubeconfig(t, fake)), stream
}

func field(t *testing.T, fake *testutil.FakeCluster, name, key string) map[string]any {
	t.Helper()
	value, ok := fake.Find(requests, name)[key].(map[string]any)
	if !ok {
		t.Fatalf("%s has no %s", name, key)
	}
	return value
}

// Seals the link as auth does, to the key the command put in the object.
func seal(t *testing.T, fake *testutil.FakeCluster, name, link string) string {
	t.Helper()
	recipient, ok := field(t, fake, name, keySpec)[keyRecipient].(string)
	key, err := base64.StdEncoding.DecodeString(recipient)
	if !ok || err != nil || len(key) != keyBytes {
		t.Fatalf("recipientKey %q is not a base64 X25519 key: %v", recipient, err)
	}
	return sealTo(t, (*[keyBytes]byte)(key), link)
}

func sealTo(t *testing.T, key *[keyBytes]byte, link string) string {
	t.Helper()
	sealed, err := box.SealAnonymous(nil, []byte(link), key, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sealed)
}

func telarkConfig(bootstrap map[string]any) *unstructured.Unstructured {
	config := &unstructured.Unstructured{Object: map[string]any{keyStatus: map[string]any{keyBootstrap: bootstrap}}}
	config.SetAPIVersion(telarkGroup)
	config.SetKind(configKind)
	config.SetNamespace(namespace)
	config.SetName(configName)
	return config
}
