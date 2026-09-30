package reshandlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/exporter/internal/constants"
	confighandler "github.com/telark/exporter/internal/handlers/config"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	kshared "github.com/telark/kcore/shared"
	xauthz "github.com/telark/x-ware/authz"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	configName     = "default"
	retryAfterSecs = 1
)

var configGR = schema.GroupResource{Group: "telark.io", Resource: "telarkconfigs"}

func TestStatusForResult(t *testing.T) {
	limiterErr := fmt.Errorf("client rate limiter Wait returned an err: %w", context.DeadlineExceeded)
	cases := []struct {
		name   string
		result kshared.KubernetesAPIData
		want   int
	}{
		{
			name:   "limiter error is unavailable, not missing",
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, limiterErr.Error(), nil, limiterErr),
			want:   http.StatusServiceUnavailable,
		},
		{
			name: "kubernetes not found",
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, constants.EmptyString,
				nil, k8serrors.NewNotFound(configGR, configName)),
			want: http.StatusNotFound,
		},
		{
			name: "real upstream status passes through",
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, constants.EmptyString,
				nil, k8serrors.NewForbidden(configGR, configName, errors.New("rbac"))),
			want: http.StatusForbidden,
		},
		{
			name:   "ok",
			result: kshared.CreateKubernetesAPIData(http.StatusOK, constants.EmptyString, &unstructured.Unstructured{}, nil),
			want:   http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sharedutils.StatusForResult(tc.result); got != tc.want {
				t.Fatalf("StatusForResult = %d, want %d", got, tc.want)
			}
		})
	}
}

// An existence check that fails for any reason but NotFound must not read as
// "gone": the caller gets the real upstream status, 503 for a transport fault.
func TestStatusForK8sError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"empty name", k8serrors.NewBadRequest("resource name cannot be empty"), http.StatusBadRequest},
		{"forbidden", k8serrors.NewForbidden(configGR, configName, errors.New("rbac")), http.StatusForbidden},
		{"throttled", k8serrors.NewTooManyRequests("throttled", retryAfterSecs), http.StatusTooManyRequests},
		{"client deadline", fmt.Errorf("get: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		if got := sharedutils.StatusForK8sError(tc.err); got != tc.want {
			t.Errorf("%s: StatusForK8sError = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// A schema rejection names the offending fields and nothing else: the API
// server's validation text carries the regex and the raw value.
func TestInvalidFieldsNamesCauses(t *testing.T) {
	invalid := k8serrors.NewInvalid(schema.GroupKind{Group: "telark.io", Kind: "TelarkConfig"}, configName, field.ErrorList{
		field.Invalid(field.NewPath("spec", "ai", "model"), "Bad Model!", "should match '^[a-z]+$'"),
		field.Invalid(field.NewPath("spec", "ai", "model"), "Bad Model!", "too long"),
		field.Required(field.NewPath("spec", "oidc", "issuer"), ""),
		field.Invalid(nil, "x", "root-level cause"),
	})
	if got, want := confighandler.InvalidFields(invalid), "spec.ai.model, spec.oidc.issuer"; got != want {
		t.Fatalf("InvalidFields = %q, want %q", got, want)
	}
	if strings.Contains(confighandler.InvalidFields(invalid), "Bad Model") {
		t.Fatal("raw value leaked")
	}
	if got := confighandler.InvalidFields(errors.New("boom")); got != constants.SpecField {
		t.Fatalf("non-status error = %q, want %q", got, constants.SpecField)
	}
	rootOnly := k8serrors.NewInvalid(schema.GroupKind{Group: "telark.io", Kind: "TelarkConfig"}, configName, field.ErrorList{
		field.Invalid(nil, "x", "root-level cause"),
	})
	if got := confighandler.InvalidFields(rootOnly); got != constants.SpecField {
		t.Fatalf("root-only causes = %q, want %q", got, constants.SpecField)
	}
}

// A key the config does not declare would be pruned by the API server and answered 200.
func TestPatchConfigRejectsUnknownKeys(t *testing.T) {
	cases := []struct {
		body    string
		refused bool
	}{
		{`{"foo":1}`, true},
		{`{"spec":{"snapshots":{"maxPerAp":3}}}`, true},
		{`{"snapshots":{"maxPerApp":3},"metadata":{"resourceVersion":"1"}}`, false},
		{`{"spec":{"oidc":{"googleJwkJson":""}},"metadata":{"resourceVersion":"1"}}`, false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodPatch, "/config", strings.NewReader(tc.body))
		r = r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{Internal: true}))
		rec := httptest.NewRecorder()
		confighandler.PatchConfig()(rec, r)
		if (rec.Code == http.StatusBadRequest) != tc.refused {
			t.Errorf("%s: code = %d, refused want %v", tc.body, rec.Code, tc.refused)
		}
	}
}

// With no cluster the client itself fails; the handler must report that as
// unavailable rather than claim the config does not exist.
func TestGetConfigWithoutClusterIsUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	confighandler.GetConfig()(rec, varsReq(http.MethodGet, ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
