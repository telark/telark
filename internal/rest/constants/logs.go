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

	// Client operation constants
	ERROR_FAILED_EXECUTE_REQUEST            errors.Error = "failed to execute request: %w"
	ERROR_FAILED_READ_RESPONSE_BODY_GENERIC errors.Error = "failed to read response body: %w"
	ERROR_UNEXPECTED_STATUS_CODE            errors.Error = "unexpected status code: %d"
	ERROR_FAILED_UNMARSHAL_RESPONSE         errors.Error = "failed to unmarshal response: %w"
	ERROR_FAILED_MARSHAL_DATA               errors.Error = "failed to marshal data: %w"
	ERROR_FAILED_UNMARSHAL_DATA             errors.Error = "failed to unmarshal data: %w"
	ERROR_FAILED_MARSHAL_ITEMS              errors.Error = "failed to marshal items: %w"
	ERROR_FAILED_UNMARSHAL_ITEMS            errors.Error = "failed to unmarshal items: %w"
	ERROR_FAILED_MARSHAL_PAYLOAD            errors.Error = "failed to marshal %s payload"
	ERROR_FAILED_GENERATE_URL               errors.Error = "failed to generate URL"
	ERROR_FAILED_SEND_REQUEST               errors.Error = "failed to send request for %s"
	ERROR_FAILED_MARSHAL_PATCH_PAYLOAD      errors.Error = "failed to marshal patch payload"
	ERROR_FAILED_CREATE_PATCH_REQUEST       errors.Error = "failed to create PATCH request"
	ERROR_FAILED_EXECUTE_PATCH_REQUEST      errors.Error = "failed to execute PATCH request"
	ERROR_FAILED_CREATE_DELETE_REQUEST      errors.Error = "failed to create DELETE request"
	ERROR_FAILED_EXECUTE_DELETE_REQUEST     errors.Error = "failed to execute DELETE request"
	ERROR_FAILED_SEND_GET_REQUEST           errors.Error = "failed to send GET request for %s %s: %w"
	ERROR_FAILED_SERIALIZE_DATA             errors.Error = "failed to serialize data for %s %s: %w"
	ERROR_FAILED_MAP_DATA_FIELD             errors.Error = "failed to map data field for %s %s: %w"
	ERROR_FAILED_CREATE_REQUEST             errors.Error = "failed to create request for %s %s: %w"
	ERROR_FAILED_GET_RESOURCE               errors.Error = "failed to get %s %s: %w"
	ERROR_FAILED_GET_RESOURCES              errors.Error = "failed to get %s: %w"
	ERROR_FAILED_CREATE_REQUEST_GENERIC     errors.Error = "failed to create request: %w"

	// Resource type constants
	RESOURCE_TYPE_APP_WORKLOAD      = "AppWorkload"
	RESOURCE_TYPE_BATCH_WORKLOAD    = "BatchWorkload"
	RESOURCE_TYPE_GROUPER           = "Grouper"
	RESOURCE_TYPE_BRIDGE            = "Bridge"
	RESOURCE_TYPE_ADMISSION_WEBHOOK = "AdmissionValidatingWebhook"
	RESOURCE_TYPE_MAINTENANCE       = "GrouperMaintenance"

	// HTTP method constants
	HTTP_METHOD_GET    = "GET"
	HTTP_METHOD_POST   = "POST"
	HTTP_METHOD_PATCH  = "PATCH"
	HTTP_METHOD_DELETE = "DELETE"

	// HTTP status constants
	HTTP_STATUS_OK                    = 200
	HTTP_STATUS_ACCEPTED              = 202
	HTTP_STATUS_INTERNAL_SERVER_ERROR = 500

	// Content type constants
	CONTENT_TYPE_JSON = "application/json"

	// Endpoint placeholder
	ENDPOINT_NAME_PLACEHOLDER = "{name}"

	// Response data keys
	RESPONSE_DATA_KEY  = "data"
	RESPONSE_ITEMS_KEY = "items"
)
