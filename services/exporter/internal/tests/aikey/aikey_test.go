package aikey

import (
	"context"
	"testing"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/secrets"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "telark"
	testName      = "telark-ai-provider-key"
	testKey       = "sk-test-0123456789"
	getErrFmt     = "Get: %v"
)

func newStore(objects ...runtime.Object) *secrets.AIKeyStore {
	client := fake.NewSimpleClientset(objects...)
	return secrets.NewAIKeyStoreWithClient(client, testNamespace, testName)
}

func existingSecret(value string) *k8scorev1.Secret {
	return &k8scorev1.Secret{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testName, Namespace: testNamespace},
		Data:       map[string][]byte{constants.AIKeySecretField: []byte(value)},
	}
}

func TestGetIsEmptyWhenSecretAbsent(t *testing.T) {
	got, err := newStore().Get(context.Background())
	if err != nil {
		t.Fatalf(getErrFmt, err)
	}
	if got != constants.EmptyString {
		t.Errorf("Get = %q, want empty when no secret exists", got)
	}
}

func TestSetThenGetRoundTrips(t *testing.T) {
	ctx := context.Background()
	store := newStore()

	if err := store.Set(ctx, testKey); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf(getErrFmt, err)
	}
	if got != testKey {
		t.Errorf("Get = %q, want %q", got, testKey)
	}
}

func TestSetReplacesAnExistingKey(t *testing.T) {
	ctx := context.Background()
	store := newStore(existingSecret("old-key"))

	if err := store.Set(ctx, testKey); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf(getErrFmt, err)
	}
	if got != testKey {
		t.Errorf("Get = %q, want %q", got, testKey)
	}
}

func TestGetIsEmptyWhenFieldMissing(t *testing.T) {
	secret := &k8scorev1.Secret{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testName, Namespace: testNamespace},
		Data:       map[string][]byte{"unrelated": []byte("x")},
	}

	got, err := newStore(secret).Get(context.Background())
	if err != nil {
		t.Fatalf(getErrFmt, err)
	}
	if got != constants.EmptyString {
		t.Errorf("Get = %q, want empty when the field is absent", got)
	}
}

func TestClearRemovesTheKey(t *testing.T) {
	ctx := context.Background()
	store := newStore(existingSecret(testKey))

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf(getErrFmt, err)
	}
	if got != constants.EmptyString {
		t.Errorf("Get = %q, want empty after Clear", got)
	}
}
