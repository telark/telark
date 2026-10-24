package writes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	createSlots     = 10 // generics' k8sCreatePoolSize
	retriedAttempts = 2  // generics' maxCreateAttempts
	oneAttempt      = 1
	heldPrefix      = "held-"
	queuedName      = "queued"
	quotaTimeout    = "resource quota evaluation timed out"
	slotTimeout     = 10 * time.Second
)

// The API server holds each create of a held- name until gate closes, after telling entered.
type heldCreates struct {
	dynamic.Interface
	entered chan<- struct{}
	gate    <-chan struct{}
}

type heldResource struct {
	dynamic.NamespaceableResourceInterface
	held heldCreates
}

type heldNamespace struct {
	dynamic.ResourceInterface
	held heldCreates
}

func (c heldCreates) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return heldResource{c.Interface.Resource(gvr), c}
}

func (r heldResource) Namespace(namespace string) dynamic.ResourceInterface {
	return heldNamespace{r.NamespaceableResourceInterface.Namespace(namespace), r.held}
}

func (n heldNamespace) Create(
	ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	if strings.HasPrefix(obj.GetName(), heldPrefix) {
		n.held.entered <- struct{}{}
		<-n.held.gate
	}
	return n.ResourceInterface.Create(ctx, obj, options, subresources...)
}

// Takes every create slot with a create the API server holds; the returned func lets them finish, each with a 200.
func holdCreateSlots(t *testing.T, client *dynamicfake.FakeDynamicClient) func() {
	t.Helper()
	entered, gate := make(chan struct{}), make(chan struct{})
	k8sclient.SetDynamicClient(heldCreates{client, entered, gate})
	codes := make(chan int, createSlots)
	for i := range createSlots {
		go func() {
			name := heldPrefix + strconv.Itoa(i)
			w := httptest.NewRecorder()
			generics.GenericCreateCustomResource(t.Context(), w, v1alpha1.PasskeyMetadata, name, map[string]any{keyName: name})
			codes <- w.Code
		}()
	}
	for range createSlots {
		select {
		case <-entered:
		case <-time.After(slotTimeout):
			t.Fatal("a create slot stayed taken")
		}
	}
	return func() {
		close(gate)
		for range createSlots {
			if code := <-codes; code != http.StatusOK {
				t.Errorf("held create = %d, want 200 once the API server answers", code)
			}
		}
	}
}

// A create that finds every slot taken gives up after CreateSlotWait, or as soon as its caller has, with a 503:
// nothing is created, and every slot is free afterwards.
func TestAQueuedCreateGivesUpWithA503(t *testing.T) {
	cases := []struct {
		name string
		gone bool
		full bool
	}{
		{"at the bound", false, true},
		{"once the caller gave up", true, true},
		{"once the caller gave up, with a slot free", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t)
			release := func() {}
			if c.full {
				release = holdCreateSlots(t, client)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if c.gone {
				cancel()
			}

			md := v1alpha1.PasskeyMetadata
			began := time.Now()
			w := httptest.NewRecorder()
			generics.GenericCreateCustomResource(ctx, w, md, queuedName, map[string]any{keyName: queuedName})
			waited := time.Since(began)
			release()

			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), string(constants.ErrCreateSlotBusy)) {
				t.Fatalf("queued create = %d %s, want 503", w.Code, w.Body.String())
			}
			if (waited >= constants.CreateSlotWait) == c.gone {
				t.Errorf("waited %s with the caller gone = %v, want the full %s only while it waits", waited, c.gone, constants.CreateSlotWait)
			}
			_, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).Get(t.Context(), queuedName, metav1.GetOptions{})
			if !k8serrors.IsNotFound(err) {
				t.Errorf("the queued create was stored (%v)", err)
			}
			holdCreateSlots(t, installFake(t))()
		})
	}
}

// A create whose attempt met the API server's quota timeout tries again only while its caller still waits.
func TestAQuotaTimeoutRetriesOnlyForAWaitingCaller(t *testing.T) {
	cases := []struct {
		name     string
		gone     bool
		attempts int
	}{
		{"caller waits", false, retriedAttempts},
		{"caller gave up meanwhile", true, oneAttempt},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var attempts int
			client.PrependReactor(verbCreate, "*", func(k8stesting.Action) (bool, runtime.Object, error) {
				attempts++
				if c.gone {
					cancel()
				}
				return true, nil, k8serrors.NewInternalError(errors.New(quotaTimeout))
			})
			generics.GenericCreateCustomResource(ctx, httptest.NewRecorder(), v1alpha1.PasskeyMetadata, queuedName,
				map[string]any{keyName: queuedName})
			if attempts != c.attempts {
				t.Errorf("%d attempts, want %d", attempts, c.attempts)
			}
		})
	}
}
