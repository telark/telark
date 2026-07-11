package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination/forcesync"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func SyncApplication(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			respondPanic(w, rec)
		}
	}()

	name, ok := readAppName(w, r)
	if !ok {
		return
	}
	ingress := getForceSyncIngress()
	if ingress == nil {
		writeQueueUnavailable(w)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ForceSyncEnqueueTimeout)
	defer cancel()

	req := forcesync.EnqueueRequest{
		AppName:     name,
		RequestedBy: strings.TrimSpace(r.Header.Get(constants.HeaderUserID)),
		Reason:      strings.TrimSpace(r.URL.Query().Get(constants.ForceSyncStreamFieldReason)),
	}
	result, err := ingress.Enqueue(ctx, req)
	if err != nil {
		writeEnqueueError(w, err)
		return
	}
	writeEnqueueResult(w, name, result)
}

func respondPanic(w http.ResponseWriter, rec any) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Error(fmt.Sprintf("%v\n%s", rec, string(debug.Stack())))
	responseutils.LogAndSendResponse(
		w,
		http.StatusInternalServerError,
		response.OperationError,
		fmt.Sprintf(string(constants.ErrForceSyncEnqueueFailed), rec),
		nil,
		fmt.Errorf("%v", rec),
	)
}

func readAppName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name, err := sharedhelper.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		return constants.EmptyString, false
	}
	name = strings.TrimSpace(name)
	if name == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w, http.StatusBadRequest, response.OperationError, string(constants.ErrAppNameRequired), nil, nil,
		)
		return constants.EmptyString, false
	}
	return name, true
}

func writeQueueUnavailable(w http.ResponseWriter) {
	payload := map[string]any{
		constants.ForceSyncResponseFieldRetryAfterSec: constants.ForceSyncRetryAfterSec,
	}
	writeJSON(w, http.StatusServiceUnavailable, response.OperationError,
		string(constants.ErrForceSyncQueueUnavailable), payload)
}

func writeEnqueueError(w http.ResponseWriter, err error) {
	responseutils.LogAndSendResponse(
		w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err,
	)
}

func writeEnqueueResult(w http.ResponseWriter, appName string, result forcesync.EnqueueResult) {
	status := http.StatusOK
	if result.Enqueued {
		status = http.StatusAccepted
	}
	payload := map[string]any{
		constants.ForceSyncResponseFieldJobID:   result.JobID,
		constants.ForceSyncResponseFieldAppName: appName,
		constants.ForceSyncResponseFieldPhase:   result.Phase,
		constants.ForceSyncResponseFieldStatus:  result.Status,
	}
	writeJSON(w, status, response.OperationSuccess, result.Status, payload)
}

func writeJSON(w http.ResponseWriter, status int, op response.OperationStatus, msg string, data map[string]any) {
	w.Header().Set("Content-Type", constants.ApplicationJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response.GenericResponse{
		Status:    status,
		Operation: string(op),
		Message:   msg,
		Data:      data,
	})
}
