package constants

import "github.com/plsyro/data-pkg/errors"

const (
	SCHEMA_IS_REQUIRED                                 = "schema is required"
	SERVICE_IS_REQUIRED                                = "service is required"
	VERSION_IS_REQUIRED                                = "version is required"
	ENDPOINT_IS_REQUIRED                               = "endpoint is required"
	VALIDATION_FAILED                                  = "validation failed: %v"
	ERROR_FAILED_GENERATE_REQUEST_URL     errors.Error = "failed to generate request URL: %v"
	ERROR_FAILED_SEND_POST_REQUEST        errors.Error = "failed to send POST request: %v"
	ERROR_FAILED_READ_RESPONSE_BODY       errors.Error = "failed to read response body: %v"
	ERROR_FAILED_PARSE_API_RESPONSE       errors.Error = "failed to parse API response: %v"
	ERROR_RESPONSE_NOT_202_STATUS         errors.Error = "response at index %d did not return status 202: got %d"
	ERROR_FAILED_GENERATE_REQUEST_URL_FOR errors.Error = "failed to generate request URL for %s '%s': %v"
	ERROR_FAILED_SEND_GET_REQUEST_FOR     errors.Error = "failed to send GET request for %s '%s': %v"
	ERROR_FAILED_READ_RESPONSE_BODY_FOR   errors.Error = "failed to read response body for %s '%s': %v"
	ERROR_FAILED_PARSE_API_RESPONSE_FOR   errors.Error = "failed to parse API response for %s '%s': %v"
	ERROR_NON_SUCCESS_STATUS_FOR          errors.Error = "received non-success status %d for %s '%s'"
	ERROR_UNEXPECTED_DATA_FIELD_FORMAT    errors.Error = "unexpected format for 'data' field"
	ERROR_ITEMS_FIELD_MISSING             errors.Error = "'items' field missing in response"
	ERROR_UNEXPECTED_STATUS               errors.Error = "unexpected status: %d"
)
