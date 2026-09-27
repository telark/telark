package reasons

import (
	"strings"

	"github.com/telark/discovery/internal/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	K8sUnavailable     = "could not deploy protection rules due to a temporary cluster issue"
	AdmissionRejected  = "policy engine rejected the protection rules; contact your platform administrator"
	UserFriendlyDeploy = "Could not deploy protection rules;" +
		" please contact your platform administrator."
)

// Transient cluster issues collapse into a retryable phrasing, admission rejections into an
// actionable one; anything else stays vague so no internal engine detail leaks to users.
func UserFacingReason(err error) string {
	if err == nil {
		return constants.EmptyString
	}
	if apierrors.IsNotFound(err) {
		return K8sUnavailable
	}
	if apierrors.IsForbidden(err) || apierrors.IsInvalid(err) {
		return AdmissionRejected
	}
	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) || apierrors.IsServiceUnavailable(err) {
		return K8sUnavailable
	}
	if strings.Contains(err.Error(), "admission webhook") {
		return AdmissionRejected
	}
	return UserFriendlyDeploy
}
