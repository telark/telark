package constants

import "github.com/plsyro/data-pkg/errors"

const (
	ERROR_FAILED_GENERATE_REQUEST_URL errors.Error = "failed to generate request URL: %v"
	ERROR_FAILED_SEND_POST_REQUEST    errors.Error = "failed to send POST request: %v"
	ERROR_UNEXPECTED_STATUS           errors.Error = "unexpected status: %d: %s"
	ERROR_FAILED_CREATE_HTTP_REQUEST  errors.Error = "create HTTP request"
	ENDPOINT_NAME_PLACEHOLDER                      = "{name}"
	SCHEMA_IS_REQUIRED                             = "schema is required"
	SERVICE_IS_REQUIRED                            = "service is required"
	VERSION_IS_REQUIRED                            = "version is required"
	ENDPOINT_IS_REQUIRED                           = "endpoint is required"
	VALIDATION_FAILED                              = "validation failed: %v"
)
