package rollbackctl

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/kcore/k8sclient"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/handlers/rollback"
	"github.com/telark/telark/services/discovery/internal/helpers/async"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	shopApp     = "shop"
	rollbackID  = "rb-1"
	triggeredBy = "user-1"
	staleAge    = 2 * time.Hour
	listSuffix  = "List"
	okBody      = `{"status":200}`
)

var appGVR = schema.GroupVersionResource{
	Group:    v1alpha1.ApplicationMetadata.BaseGroup,
	Version:  v1alpha1.ApplicationMetadata.Version,
	Resource: v1alpha1.ApplicationMetadata.Plural,
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type sentNotifications struct {
	mu   sync.Mutex
	sent []notifclient.Notification
}

func (s *sentNotifications) drained() []notifclient.Notification {
	async.Drain()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

// The notifier host is a fixed cluster DNS name, so emits are captured by
// swapping the default transport the rest client dials through.
func captureNotifications(t *testing.T) *sentNotifications {
	t.Helper()
	async.Init()
	out := &sentNotifications{}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var n notifclient.Notification
		if err := json.NewDecoder(r.Body).Decode(&n); err == nil {
			out.mu.Lock()
			out.sent = append(out.sent, n)
			out.mu.Unlock()
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(okBody))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	return out
}

func installApplication(t *testing.T, entry application.RollbackEntry) (*dynamicfake.FakeDynamicClient, *application.Application) {
	t.Helper()
	spec := &application.Application{Name: shopApp, Rollbacks: []application.RollbackEntry{entry}}
	rollbacks, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&struct {
		Rollbacks []application.RollbackEntry `json:"rollbacks"`
	}{spec.Rollbacks})
	if err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":                appGVR.GroupVersion().String(),
		"kind":                      v1alpha1.ApplicationMetadata.Kind,
		"metadata":                  map[string]any{"name": shopApp, "namespace": v1alpha1.ApplicationMetadata.Namespace},
		constants.RollbackStatusKey: rollbacks,
	}}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{appGVR: v1alpha1.ApplicationMetadata.Kind + listSuffix}, obj)
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	return client, spec
}

func storedRollback(t *testing.T, client *dynamicfake.FakeDynamicClient) application.RollbackEntry {
	t.Helper()
	obj, err := client.Resource(appGVR).Namespace(v1alpha1.ApplicationMetadata.Namespace).
		Get(context.Background(), shopApp, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := unstructured.NestedMap(obj.Object, constants.RollbackStatusKey)
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Rollbacks []application.RollbackEntry `json:"rollbacks"`
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &status); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "one stored rollback", len(status.Rollbacks), constants.DefaultAddValue)
	return status.Rollbacks[constants.DefaultInitValue]
}

// An entry stuck in_progress past the stale window was relabeled failed without a
// word to the user who triggered it; every other failure path notifies them.
func TestFailStaleInProgressNotifiesTheTrigger(t *testing.T) {
	sent := captureNotifications(t)
	client, spec := installApplication(t, application.RollbackEntry{
		ID:               rollbackID,
		TargetGeneration: constants.ThreeValue,
		TriggeredBy:      triggeredBy,
		TriggeredAt:      time.Now().Add(-staleAge),
		Status:           constants.RollbackStatusInProgress,
	})

	handled, err := rollback.NewController(nil).FailStaleInProgress(context.Background(), shopApp, spec)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "handled", handled, true)
	testutil.Equal(t, "stored status", storedRollback(t, client).Status, constants.RollbackStatusFailed)

	got := sent.drained()
	testutil.Equal(t, "one notification", len(got), constants.DefaultAddValue)
	n := got[constants.DefaultInitValue]
	testutil.Equal(t, "recipient", n.UserID, triggeredBy)
	testutil.Equal(t, "severity", n.Severity, notifclient.SeverityError)
	testutil.Equal(t, "status", n.Metadata[notifclient.MetaKeyStatus], any(notifclient.RollbackStatusFailure))
	testutil.Equal(t, "applicationId is the app", n.Metadata[notifclient.MetaKeyApplicationID], any(shopApp))
	testutil.Equal(t, "reason in message", strings.Contains(n.Message, string(constants.ErrRollbackInterruptedRestart)), true)
}

// restoredGeneration was declared on the CRD but never written, so the rollbacks
// panel never said which generation a successful rollback brought back.
func TestFinalizeRollbackSuccessRecordsRestoredGeneration(t *testing.T) {
	sent := captureNotifications(t)
	client, spec := installApplication(t, application.RollbackEntry{
		ID:               rollbackID,
		TargetGeneration: constants.ThreeValue,
		TriggeredBy:      triggeredBy,
		TriggeredAt:      time.Now(),
		Status:           constants.RollbackStatusInProgress,
	})

	pending := spec.Rollbacks[constants.DefaultInitValue]
	err := rollback.NewController(nil).FinalizeRollbackSuccess(
		context.Background(), shopApp, spec, &pending, constants.DefaultInitValue)
	if err != nil {
		t.Fatal(err)
	}
	stored := storedRollback(t, client)
	testutil.Equal(t, "stored status", stored.Status, constants.RollbackStatusSuccess)
	testutil.Equal(t, "restoredGeneration written", stored.RestoredGeneration != nil, true)
	testutil.Equal(t, "restoredGeneration", *stored.RestoredGeneration, constants.ThreeValue)

	got := sent.drained()
	testutil.Equal(t, "one notification", len(got), constants.DefaultAddValue)
	testutil.Equal(t, "applicationId is the app",
		got[constants.DefaultInitValue].Metadata[notifclient.MetaKeyApplicationID], any(shopApp))
}
