package clients

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/constants"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
)

// The connectivity gate returns a plain fmt.Errorf with no sentinel, so its
// rest-pkg format string is the only thing left to recognise it by.
var connectivityNotReadyPrefix, _, _ = strings.Cut(
	string(restconstants.ErrConnectivityServiceNotReady),
	constants.FormatVerbPrefix,
)

// rest-pkg preserves the real status on responses and synthesizes 500 only for
// transport or gate failures, so 500 is the single status worth tripping on.
func classifyStatus(status int, err error) error {
	if err == nil || status == http.StatusInternalServerError {
		return err
	}
	return circuitbreaker.NotCounted(err)
}

func classifyTransport(err error) error { return classifyWrapped(err, err) }

// Fail-safe for paths that fold the status into a format string: only a proven
// connectivity failure counts, so routine 4xx can never open the circuit.
func classifyWrapped(cause, out error) error {
	if out == nil || isConnectivityFailure(cause) {
		return out
	}
	return circuitbreaker.NotCounted(out)
}

func isConnectivityFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return strings.HasPrefix(err.Error(), connectivityNotReadyPrefix)
}

func classifyResponse(resp *response.GenericResponse) error {
	if resp == nil {
		return errors.New(string(constants.ErrPatchApplicationReturnedNilResponse))
	}
	if resp.Status == http.StatusOK {
		return nil
	}
	return classifyStatus(resp.Status, fmt.Errorf(
		string(constants.ErrRestCallFailed), resp.Status, resp.Message,
	))
}

// An open circuit leaves resp nil; callers expect a response value, so the
// short-circuit is surfaced the same way rest-pkg surfaces a transport failure.
func guardedResponse(op func() *response.GenericResponse) *response.GenericResponse {
	var resp *response.GenericResponse
	err := circuitbreaker.ExecuteExporter(func() error {
		resp = op()
		return classifyResponse(resp)
	})
	if resp == nil {
		return &response.GenericResponse{
			Status:    http.StatusInternalServerError,
			Operation: string(response.OperationError),
			Message:   err.Error(),
		}
	}
	return resp
}

func guardedStatusError(failFormat dataerrors.Error, op func() *response.GenericResponse) error {
	return circuitbreaker.ExecuteExporter(func() error {
		resp := op()
		if resp == nil {
			return errors.New(string(constants.ErrPatchApplicationReturnedNilResponse))
		}
		if resp.Status == http.StatusOK {
			return nil
		}
		return classifyStatus(resp.Status, fmt.Errorf(
			string(failFormat), resp.Status, resp.Message,
		))
	})
}

func guardedExporterGet[T any](op func() (T, error)) (T, error) {
	var out T
	err := circuitbreaker.ExecuteExporter(func() error {
		result, callErr := op()
		if callErr != nil {
			return classifyTransport(callErr)
		}
		out = result
		return nil
	})
	return out, err
}
