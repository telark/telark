package constants

import "github.com/plsyro/data/errors"

const (
	ErrFailedToGenerateRequestURL errors.Error = "failed to generate request URL: %v"
	ErrFailedToSendPostRequest    errors.Error = "failed to send POST request: %v"
	ErrUnexpectedStatus           errors.Error = "unexpected status: %d: %s"
	ErrFailedToCreateHTTPRequest  errors.Error = "create HTTP request"
	ErrFailedToCloseResponseBody  errors.Error = "failed to close response body: %v"
	ErrSchemaIsRequired           errors.Error = "schema is required"
	ErrServiceIsRequired          errors.Error = "service is required"
	ErrVersionIsRequired          errors.Error = "version is required"
	ErrEndpointIsRequired         errors.Error = "endpoint is required"
	ErrValidationFailed           errors.Error = "validation failed: %v"
	EndpointNamePlaceholder       errors.Error = "{name}"
)
