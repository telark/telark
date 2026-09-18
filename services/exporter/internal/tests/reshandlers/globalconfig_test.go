package reshandlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	globalconfighandler "github.com/telark/exporter/internal/handlers/resources/globalconfig"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	kshared "github.com/telark/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, "",
				nil, k8serrors.NewNotFound(globalConfigGR, "global-config")),
			want: http.StatusNotFound,
		},
		{
			name: "real upstream status passes through",
			result: kshared.CreateKubernetesAPIData(http.StatusInternalServerError, "",
				nil, k8serrors.NewForbidden(globalConfigGR, "global-config", errors.New("rbac"))),
			want: http.StatusForbidden,
		},
		{
			name:   "ok",
			result: kshared.CreateKubernetesAPIData(http.StatusOK, "", &unstructured.Unstructured{}, nil),
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

// With no cluster the client itself fails; the handler must report that as
// unavailable rather than claim the global config does not exist.
func TestGetGlobalConfigWithoutClusterIsUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	globalconfighandler.GetGlobalConfig()(rec, varsReq(http.MethodGet, ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
