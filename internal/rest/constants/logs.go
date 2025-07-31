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
	ERROR_FAILED_PARSE_API_RESPONSE       errors.Error = "failed to parse API response: %v"
	ERROR_RESPONSE_NOT_202_STATUS         errors.Error = "response at index %d did not return status 202: got %d"
	ERROR_FAILED_GENERATE_REQUEST_URL_FOR errors.Error = "failed to generate request URL for %s '%s': %v"
	ERROR_FAILED_SEND_GET_REQUEST_FOR     errors.Error = "failed to send GET request for %s '%s': %v"
	ERROR_FAILED_PARSE_API_RESPONSE_FOR   errors.Error = "failed to parse API response for %s '%s': %v"
	ERROR_NON_SUCCESS_STATUS_FOR          errors.Error = "received non-success status %d for %s '%s'"
	ERROR_UNEXPECTED_DATA_FIELD_FORMAT    errors.Error = "unexpected format for 'data' field"
	ERROR_ITEMS_FIELD_MISSING             errors.Error = "'items' field missing in response"
	ERROR_UNEXPECTED_STATUS               errors.Error = "unexpected status: %d: %s"

	// Client operation constants
	ERROR_FAILED_EXECUTE_REQUEST        errors.Error = "failed to execute request: %w"
	ERROR_UNEXPECTED_STATUS_CODE        errors.Error = "unexpected status code: %d"
	ERROR_FAILED_MARSHAL_DATA           errors.Error = "failed to marshal data: %w"
	ERROR_FAILED_MARSHAL_PAYLOAD        errors.Error = "failed to marshal %s payload"
	ERROR_FAILED_GENERATE_URL           errors.Error = "failed to generate URL"
	ERROR_FAILED_SEND_REQUEST           errors.Error = "failed to send request for %s"
	ERROR_FAILED_CREATE_PATCH_REQUEST   errors.Error = "failed to create PATCH request"
	ERROR_FAILED_EXECUTE_PATCH_REQUEST  errors.Error = "failed to execute PATCH request"
	ERROR_FAILED_CREATE_DELETE_REQUEST  errors.Error = "failed to create DELETE request"
	ERROR_FAILED_EXECUTE_DELETE_REQUEST errors.Error = "failed to execute DELETE request"
	ERROR_FAILED_SEND_GET_REQUEST       errors.Error = "failed to send GET request for %s %s: %w"
	ERROR_FAILED_SERIALIZE_DATA         errors.Error = "failed to serialize data for %s %s: %w"
	ERROR_FAILED_MAP_DATA_FIELD         errors.Error = "failed to map data field for %s %s: %w"
	ERROR_FAILED_CREATE_REQUEST         errors.Error = "failed to create request for %s %s: %w"
	ERROR_FAILED_GET_RESOURCE           errors.Error = "failed to get %s %s: %w"
	ERROR_FAILED_GET_RESOURCES          errors.Error = "failed to get %s: %w"
	ERROR_FAILED_CREATE_REQUEST_GENERIC errors.Error = "failed to create request: %w"

	// Client operation error messages
	ERROR_FAILED_MARSHAL_RESOURCE    errors.Error = "marshal resource"
	ERROR_FAILED_CREATE_RESOURCE     errors.Error = "create resource"
	ERROR_FAILED_MARSHAL_UPDATE_BODY errors.Error = "marshal update body"
	ERROR_FAILED_UPDATE_RESOURCE     errors.Error = "update resource"
	ERROR_FAILED_DELETE_RESOURCE     errors.Error = "delete resource"

	// Utility operation error messages
	ERROR_FAILED_BUILD_REQUEST_URL   errors.Error = "build request URL"
	ERROR_FAILED_CREATE_HTTP_REQUEST errors.Error = "create HTTP request"

	// Endpoint placeholder
	ENDPOINT_NAME_PLACEHOLDER = "{name}"
)
