package planreasons

import (
	"errors"
	"testing"

	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reasons"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const objectName = "p"

// UserFacingReason collapses each error class into its stable user-facing phrase.
func TestUserFacingReason(t *testing.T) {
	gr := schema.GroupResource{Group: "kyverno.io", Resource: "policies"}
	gk := schema.GroupKind{Group: "kyverno.io", Kind: "Policy"}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"not found", apierrors.NewNotFound(gr, objectName), reasons.K8sUnavailable},
		{"forbidden", apierrors.NewForbidden(gr, objectName, errors.New("no")), reasons.AdmissionRejected},
		{"invalid", apierrors.NewInvalid(gk, objectName, field.ErrorList{}), reasons.AdmissionRejected},
		{"timeout", apierrors.NewTimeoutError("slow", constants.DefaultInitValue), reasons.K8sUnavailable},
		{"server timeout", apierrors.NewServerTimeout(gr, "create", constants.DefaultInitValue), reasons.K8sUnavailable},
		{"unavailable", apierrors.NewServiceUnavailable("down"), reasons.K8sUnavailable},
		{"webhook text", errors.New("admission webhook denied the request"), reasons.AdmissionRejected},
		{"generic", errors.New("something else"), reasons.UserFriendlyDeploy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "reason", reasons.UserFacingReason(c.err), c.want)
		})
	}
}
