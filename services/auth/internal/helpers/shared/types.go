package shared

import restresponse "github.com/telark/telark/internal/rest/response"

type (
	RefusalError struct {
		Code string
		Err  error
	}

	RefusalResponse struct {
		restresponse.GenericResponse
		Code string `json:"code,omitempty"`
	}
)
