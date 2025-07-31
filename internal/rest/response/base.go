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
	OPERATION_SUCCESS       OperationStatus = "Success"
	OPERATION_ALREADY_EXIST OperationStatus = "Already Exists"
	OPERATION_NOT_CREATED   OperationStatus = "Not Created"
	OPERATION_NOT_UPDATED   OperationStatus = "Not Updated"
	OPERATION_NOT_FOUND     OperationStatus = "Not Found"
	OPERATION_DELETED       OperationStatus = "Deleted"
	OPERATION_UNPROCESSED   OperationStatus = "Unprocessed"
	OPERATION_ERROR         OperationStatus = "Error"
)

type GenericResponse struct {
	Status    int         `json:"status"`
	Operation string      `json:"operation,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func NewGenericResponse(status int, operation OperationStatus, data interface{}, message string) *GenericResponse {
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
		http.Error(w, fmt.Sprintf(string(errors.ERROR_REST_ENCODE_RESPONSE), err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf(string(errors.ERROR_REST_ENCODE_RESPONSE), err), http.StatusInternalServerError)
		return
	}
}

func SendSingleResponse(w http.ResponseWriter, response *GenericResponse) {
	EncodeJSONResponse(w, response.Status, response)
}

func SendMultiResponses(w http.ResponseWriter, status int, responses []*GenericResponse) {
	EncodeMultiJSONResponse(w, status, responses)
}
