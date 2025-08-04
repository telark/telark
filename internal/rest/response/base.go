package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
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
	OperationError         OperationStatus = "Error"
)

type GenericResponse struct {
	Status    int    `json:"status"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data,omitempty"`
}

func NewGenericResponse(status int, operation OperationStatus, data any, message string) *GenericResponse {
	return &GenericResponse{
		Status:    status,
		Operation: string(operation),
		Data:      data,
		Message:   message,
	}
}

func EncodeJSONResponse(w http.ResponseWriter, status int, response *GenericResponse) {
	w.Header().Set("Content-Type", string(base.JSON))
	w.WriteHeader(status)

	if response == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		http.Error(w, fmt.Sprintf(string(errors.ErrRestEncodeResponse), err), http.StatusInternalServerError)
	}
}

func EncodeMultiJSONResponse(w http.ResponseWriter, status int, responses []*GenericResponse) {
	w.Header().Set("Content-Type", string(base.JSON))

	w.WriteHeader(status)
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responsesToEncode := make([]*GenericResponse, len(responses))
	copy(responsesToEncode, responses)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(responsesToEncode); err != nil {
		http.Error(w, fmt.Sprintf(string(errors.ErrRestEncodeResponse), err), http.StatusInternalServerError)
		return
	}
}

func SendSingleResponse(w http.ResponseWriter, response *GenericResponse) {
	EncodeJSONResponse(w, response.Status, response)
}

func SendMultiResponses(w http.ResponseWriter, status int, responses []*GenericResponse) {
	EncodeMultiJSONResponse(w, status, responses)
}
