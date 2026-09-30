package status

import (
	"net/http"
	"strings"

	"github.com/telark/rest/response"
)

type (
	ReadyFunc func(r *http.Request) bool
	Messages  struct {
		Ready    string
		NotReady string
	}
)

func NewProbeHandler(readinessSuffix string, ready ReadyFunc, messages Messages) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, readinessSuffix) && ready != nil && !ready(r) {
			respond(w, http.StatusServiceUnavailable, response.OperationUnprocessed, messages.NotReady)
			return
		}

		respond(w, http.StatusOK, response.OperationSuccess, messages.Ready)
	}
}

func respond(w http.ResponseWriter, status int, operation response.OperationStatus, message string) {
	response.SendSingleResponse(w, response.NewGenericResponse(status, operation, nil, message))
}
