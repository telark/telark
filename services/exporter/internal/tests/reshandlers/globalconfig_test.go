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
	globalconfighandler "github.com/telark/exporter/internal/handlers/resources/globalconfig"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	kshared "github.com/telark/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	globalConfigName = "global-config"
	retryAfterSecs   = 1
)

var globalConfigGR = schema.GroupResource{Group: "telark.io", Resource: "globalconfigs"}

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
				nil, k8serrors.NewNotFound(globalConfigGR, "global-config")),
			want: http.StatusNotFound,
		},
		{
			name: "real upstream status passes through",
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, constants.EmptyString,
				nil, k8serrors.NewForbidden(globalConfigGR, "global-config", errors.New("rbac"))),
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
		{"forbidden", k8serrors.NewForbidden(globalConfigGR, globalConfigName, errors.New("rbac")), http.StatusForbidden},
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
	invalid := k8serrors.NewInvalid(schema.GroupKind{Group: "telark.io", Kind: "GlobalConfig"}, globalConfigName, field.ErrorList{
		field.Invalid(field.NewPath("spec", "ai", "model"), "Bad Model!", "should match '^[a-z]+$'"),
		field.Invalid(field.NewPath("spec", "ai", "model"), "Bad Model!", "too long"),
		field.Required(field.NewPath("spec", "oidc", "issuer"), ""),
	})
	if got, want := globalconfighandler.InvalidFields(invalid), "spec.ai.model, spec.oidc.issuer"; got != want {
		t.Fatalf("InvalidFields = %q, want %q", got, want)
	}
	if strings.Contains(globalconfighandler.InvalidFields(invalid), "Bad Model") {
		t.Fatal("raw value leaked")
	}
	if got := globalconfighandler.InvalidFields(errors.New("boom")); got != constants.SpecField {
		t.Fatalf("non-status error = %q, want %q", got, constants.SpecField)
	}
}

// With no cluster the client itself fails; the handler must report that as
// unavailable rather than claim the global config does not exist.
func TestGetGlobalConfigWithoutClusterIsUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	globalconfighandler.GetGlobalConfig()(rec, varsReq(http.MethodGet, ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
