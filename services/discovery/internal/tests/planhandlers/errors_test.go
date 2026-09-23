package planhandlers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	handlers "github.com/telark/discovery/internal/handlers/plans/protection"
	"github.com/telark/discovery/internal/tests/testutil"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Every domain failure carries its own type, so the status comes from the error itself and
// never from matching its message.
func TestStatusForErr(t *testing.T) {
	k8sErr := k8serrors.NewConflict(schema.GroupResource{Group: "kyverno.io", Resource: "policies"}, "pol-a", errors.New("conflict"))
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, http.StatusOK},
		{"not found", fmt.Errorf("get plan: %w", clients.ErrPlanNotFound), http.StatusNotFound},
		{"validation", validation.Invalid("bad request"), http.StatusBadRequest},
		{"wrapped validation", fmt.Errorf("prepare: %w", validation.ErrPoliciesRequired), http.StatusBadRequest},
		{"breaker open", fmt.Errorf("list plans: %w", circuitbreaker.ErrOpen), http.StatusServiceUnavailable},
		{"cluster error", fmt.Errorf("apply: %w", k8sErr), http.StatusServiceUnavailable},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "status", handlers.StatusForErr(c.err), c.want)
		})
	}
}
