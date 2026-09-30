package protection

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/services/discovery/internal/circuitbreaker"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// The domain returns typed errors so the API can separate a caller mistake from an exporter
// or cluster outage without matching on message text.
func StatusForErr(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, clients.ErrPlanNotFound):
		return http.StatusNotFound
	case errors.Is(err, protection.ErrDecisionSelf), validation.IsForbidden(err):
		return http.StatusForbidden
	case errors.Is(err, protection.ErrDecisionNotPending), errors.Is(err, protection.ErrDecisionStale),
		errors.Is(err, protection.ErrNameInFlight), validation.IsConflict(err):
		return http.StatusConflict
	case validation.IsValidation(err):
		return http.StatusBadRequest
	case circuitbreaker.IsOpen(err), clients.IsExporterFailure(err), isClusterError(err),
		errors.Is(err, protection.ErrCoordinationUnavailable), validation.IsUnavailable(err):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func isClusterError(err error) bool {
	var status k8serrors.APIStatus
	return errors.As(err, &status)
}

func respondDomainError(w http.ResponseWriter, err error) {
	respondError(w, StatusForErr(err), dataerrors.Error(err.Error()), err)
}

// One load per request: every handler works off the returned pointer instead of re-reading
// the global, which the bootstrap may replace mid-request.
func readyService(w http.ResponseWriter) (*protection.Service, bool) {
	svc := globalService.Load()
	if svc == nil {
		respondError(w, http.StatusServiceUnavailable, protection.ErrServiceNotReady, nil)
		return nil, false
	}
	return svc, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return constants.EmptyString, false
	}
	return userID, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	return acceptBody(w, shared.DecodeJSONStrict(w, r, target))
}

// An absent body is a valid request for the optional-payload routes, so only a malformed one
// is rejected.
func decodeOptionalBody(w http.ResponseWriter, r *http.Request, target any) bool {
	err := shared.DecodeJSONStrict(w, r, target)
	if errors.Is(err, io.EOF) {
		return true
	}
	return acceptBody(w, err)
}

func acceptBody(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	status := http.StatusBadRequest
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		status = http.StatusRequestEntityTooLarge
	}
	msg := fmt.Sprintf(string(protection.ErrRequestBody), err)
	respondError(w, status, dataerrors.Error(msg), err)
	return false
}
