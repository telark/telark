package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/constants"
)

type OperationStatus string

const (
	OperationSuccess       OperationStatus = "Success"
	OperationAlreadyExists OperationStatus = "Already Exists"
	OperationNotCreated    OperationStatus = "Not Created"
	OperationNotUpdated    OperationStatus = "Not Updated"
	OperationNotFound      OperationStatus = "Not Found"
	OperationDeleted       OperationStatus = "Deleted"
	OperationUnprocessed   OperationStatus = "Unprocessed"
	OperationUnauthorized  OperationStatus = "Unauthorized"
	OperationForbidden     OperationStatus = "Forbidden"
	OperationUnavailable   OperationStatus = "Unavailable"
	OperationError         OperationStatus = "Error"
)

type GenericResponse struct {
	Status    int    `json:"status"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data,omitempty"`
}

func NewGenericResponse(
	status int,
	operation OperationStatus,
	data any,
	message string,
) *GenericResponse {
	return &GenericResponse{
		Status:    status,
		Operation: string(operation),
		Data:      data,
		Message:   message,
	}
}

func EncodeJSONResponse(w http.ResponseWriter, status int, response *GenericResponse) {
	if response == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, status, response)
}

func EncodeMultiJSONResponse(w http.ResponseWriter, status int, responses []*GenericResponse) {
	if len(responses) == constants.EmptySliceLength {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, status, responses)
}

// Encoding happens before WriteHeader so a failure can still send its own 500.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		http.Error(
			w,
			fmt.Sprintf(string(errors.ErrRestEncodeResponse), err),
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", string(base.JSON))
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		base.GetLogger().Warn(fmt.Sprintf(string(errors.ErrRestEncodeResponse), err))
	}
}

func SendSingleResponse(w http.ResponseWriter, response *GenericResponse) {
	EncodeJSONResponse(w, response.Status, response)
}

func SendMultiResponses(w http.ResponseWriter, status int, responses []*GenericResponse) {
	EncodeMultiJSONResponse(w, status, responses)
}
