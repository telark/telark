package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	statushandler "github.com/telark/telark/services/exporter/internal/handlers/status"
	"github.com/telark/telark/services/exporter/internal/informers"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	k8scache "k8s.io/client-go/tools/cache"
)

type readiness struct {
	code    int
	message string
	data    statushandler.Diagnostics
}

func probe(t *testing.T) readiness {
	t.Helper()
	rec := httptest.NewRecorder()
	statushandler.Readiness(rec, httptest.NewRequest(http.MethodGet, "/status/ready", nil))
	var envelope response.GenericResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("readiness body: %v: %s", err, rec.Body.String())
	}
	raw, err := json.Marshal(envelope.Data)
	if err != nil {
		t.Fatal(err)
	}
	var data statushandler.Diagnostics
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("readiness data: %v: %s", err, raw)
	}
	return readiness{code: rec.Code, message: envelope.Message, data: data}
}

func useRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	exprdb.Set(client)
	t.Cleanup(func() {
		exprdb.Set(nil)
		_ = client.Close()
	})
	return mr
}

func useStore(t *testing.T, synced bool) {
	t.Helper()
	informers.Use(k8scache.NewStore(k8scache.MetaNamespaceKeyFunc), func() bool { return synced })
	t.Cleanup(func() { informers.Use(nil, nil) })
}

// No apiserver is reachable from a unit test: that is reported as degraded on a
// 200, because it fails cluster-wide and must not empty the Service.
func TestReadinessReportsApiserverAsDegraded(t *testing.T) {
	useRedis(t)
	useStore(t, true)
	got := probe(t)
	if got.code != http.StatusOK || got.message != string(constants.InfSuccessStatusMessage) {
		t.Fatalf("readiness = %d %q, want 200 %q", got.code, got.message, constants.InfSuccessStatusMessage)
	}
	if !got.data.Degraded || !slices.Contains(got.data.Reasons, constants.ReadinessReasonKubernetes) {
		t.Errorf("diagnostics = %+v, want degraded by %q", got.data, constants.ReadinessReasonKubernetes)
	}
}

// Redis and the watch cache are this replica's own: without either it is not ready.
func TestReadinessGatesOnRedisAndInformer(t *testing.T) {
	useStore(t, true)
	useRedis(t).Close()
	got := probe(t)
	if got.code != http.StatusServiceUnavailable || !slices.Contains(got.data.Reasons, constants.ReadinessReasonRedis) {
		t.Errorf("redis down: readiness = %d %+v, want 503 with %q", got.code, got.data, constants.ReadinessReasonRedis)
	}

	useRedis(t)
	useStore(t, false)
	got = probe(t)
	if got.code != http.StatusServiceUnavailable || !slices.Contains(got.data.Reasons, constants.ReadinessReasonInformer) {
		t.Errorf("informer unsynced: readiness = %d %+v, want 503 with %q", got.code, got.data, constants.ReadinessReasonInformer)
	}
	if got.message != string(constants.InfNotReadyStatusMessage) {
		t.Errorf("message = %q, want %q", got.message, constants.InfNotReadyStatusMessage)
	}
}
