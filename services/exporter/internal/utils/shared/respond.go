package shared

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	kshared "github.com/telark/kcore/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var lg = constants.GetLogger(constants.PrefixShared)

func (e *UpstreamError) Error() string {
	if e.Err == nil {
		return http.StatusText(e.Status)
	}
	return e.Err.Error()
}

func (e *UpstreamError) Unwrap() error {
	return e.Err
}

// kcore stamps every failed call 500, so only the Kubernetes error itself tells
// a missing resource from a limiter, timeout or transport fault.
func StatusForResult(result kshared.KubernetesAPIData) int {
	if result.Status == http.StatusNotFound || k8serrors.IsNotFound(result.Error) {
		return http.StatusNotFound
	}
	var apiStatus k8serrors.APIStatus
	if errors.As(result.Error, &apiStatus) && apiStatus.Status().Code != constants.DefaultInitValue {
		return int(apiStatus.Status().Code)
	}
	if result.Error != nil && result.Status == http.StatusInternalServerError {
		return http.StatusServiceUnavailable
	}
	return result.Status
}

func StatusForK8sError(err error) int {
	return StatusForResult(kshared.CreateKubernetesAPIData(http.StatusInternalServerError, constants.EmptyString, nil, err))
}

func ErrorForResult(result kshared.KubernetesAPIData, notFoundErr dataerrors.Error) error {
	switch status := StatusForResult(result); status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return errors.New(string(notFoundErr))
	default:
		return &UpstreamError{Status: status, Err: result.Error}
	}
}

func StatusForError(err error, fallback int) int {
	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		return upstream.Status
	}
	return fallback
}

// responseutils logs at Error whenever err is non-nil, so a client-side failure
// is reported here and the error is withheld from that call.
func LogByStatusAndSend(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) {
	if err != nil && status < http.StatusInternalServerError {
		lg.Warn(fmt.Sprintf(constants.LogMessageWithError, message, err))
		err = nil
	}
	responseutils.LogAndSendResponse(w, status, operation, message, data, err)
}

func LogDebugAndSend(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) {
	if err != nil {
		lg.Debug(fmt.Sprintf(constants.LogMessageWithError, message, err))
	}
	responseutils.LogAndSendResponse(w, status, operation, message, data, nil)
}
