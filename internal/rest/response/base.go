package response

import (
	"encoding/json"
	"net/http"
)

// OperationStatus represents the status of an operation in a response.
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

// GenericResponse represents a standard API response structure.
type GenericResponse struct {
	Status    int         `json:"status"`
	Operation string      `json:"operation,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// NewGenericResponse creates a new GenericResponse instance.
func NewGenericResponse(status int, operation OperationStatus, data interface{}, message string) *GenericResponse {
	return &GenericResponse{
		Status:    status,
		Operation: string(operation),
		Data:      data,
		Message:   message,
	}
}

// EncodeJSONResponse writes a single GenericResponse as JSON to the http.ResponseWriter.
func EncodeJSONResponse(w http.ResponseWriter, status int, response *GenericResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if response == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		http.Error(w, "Error On Encoding JSON Response", http.StatusInternalServerError)
	}
}

// EncodeMultiJSONResponse writes multiple GenericResponses as a JSON array to the http.ResponseWriter.
// Uses json.Encoder for efficient streaming.
func EncodeMultiJSONResponse(w http.ResponseWriter, status int, responses []*GenericResponse) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Create a slice to hold the encoded responses
	responsesToEncode := make([]*GenericResponse, len(responses))
	copy(responsesToEncode, responses)

	// Encode the responses as a single JSON array
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(responsesToEncode); err != nil {
		http.Error(w, "Error On Encoding JSON Response", http.StatusInternalServerError)
		return
	}
}

// SendSingleResponse sends a single GenericResponse using EncodeJSONResponse.
func SendSingleResponse(w http.ResponseWriter, response *GenericResponse) {
	EncodeJSONResponse(w, response.Status, response)
}

// SendMultiResponses sends multiple GenericResponses using EncodeMultiJSONResponse.
func SendMultiResponses(w http.ResponseWriter, status int, responses []*GenericResponse) {
	EncodeMultiJSONResponse(w, status, responses)
}
