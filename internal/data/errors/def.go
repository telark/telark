package errors

type Error string

const (
	// REST-related Errors
	ERROR_REST_MARSHALL_PAYLOAD               Error = "Error while marshaling JSON payload."
	ERROR_REST_UNMARSHALL_RESPONSE_TO_GENERIC Error = "Error while unmarshaling JSON Response to Generic Response."
	ERROR_REST_SEND_REQUEST                   Error = "Error while sending request."
	ERROR_REST_READ_RESPONSE_BODY             Error = "Error while reading response body."
	ERROR_REST_READ_REQUEST_BODY              Error = "Error while reading request body."
	ERROR_REST_MISSING_REQUEST_BODY           Error = "Missing request body for the '{action}' action."
	ERROR_REST_PARSE_REQUEST_BODY             Error = "Error while parsing request body."
	ERROR_REST_DECODE_RESPONSE                Error = "Error while decoding response."
	ERROR_REST_MISSING_PARAM                  Error = "The '{param}' parameter is required in the URL path."

	// K8s-related Errors
	ERROR_CLIENT_KUBE Error = "Failed to get Kubernetes client."

	// Resource-related Errors
	ERROR_CREATE_RESOURCE              Error = "Error while creating resource."
	ERROR_GET_RESOURCE                 Error = "Error while fetching resource."
	ERROR_LIST_RESOURCES               Error = "Error while listing resources."
	ERROR_UPDATE_RESOURCE              Error = "Error while updating resource."
	ERROR_DELETE_RESOURCE              Error = "Error while deleting resource."
	ERROR_UPDATE_RESOURCE_WITH_HISTORY Error = "Error while updating resource with history."
	ERROR_UPDATE_RESOURCE_SYNC         Error = "Error while updating resource sync settings."
	ERROR_FILTER_RESOURCES             Error = "Error while filtering resources."
	ERROR_FILTER_RESOURCE              Error = "Error while filtering resource."
	ERROR_CHECK_RESOURCE_EXISTENCE     Error = "Error while checking resource existence."
	ERROR_RESOURCE_EXISTS              Error = "Resource already exists."
	ERROR_RESOURCE_NOT_FOUND           Error = "Resource not found."

	// Admission-related Errors
	ERROR_CREATE_VALIDATING_ADMISSION Error = "Error while creating admission validating webhook."
	ERROR_CREATE_MUTATING_ADMISSION   Error = "Error while creating admission mutating webhook."
	ERROR_GET_VALIDATING_ADMISSION    Error = "Error while fetching admission validating webhook."
	ERROR_GET_MUTATING_ADMISSION      Error = "Error while fetching admission mutating webhook."
	ERROR_UPDATE_VALIDATING_ADMISSION Error = "Error while updating admission validating webhook."
	ERROR_UPDATE_MUTATING_ADMISSION   Error = "Error while updating admission mutating webhook."
	ERROR_DELETE_VALIDATING_ADMISSION Error = "Error while deleting admission validating webhook."
	ERROR_DELETE_MUTATING_ADMISSION   Error = "Error while deleting admission mutating webhook."
	ERROR_FILTER_ADMISSION            Error = "Error while filtering admission webhook."
	ERROR_CHECK_ADMISSION_EXISTENCE   Error = "Error while checking admission webhook existence."
	ERROR_ADMISSION_VALIDATING_EXISTS Error = "Admission validating webhook already exists."
	ERROR_ADMISSION_MUTATING_EXISTS   Error = "Admission mutating webhook already exists."
	ERROR_ADMISSION_NOT_FOUND         Error = "Admission webhook not found."

	// NATS-related Errors
	ERROR_NATS_CONNECTION      Error = "Failed to connect to NATS server."
	ERROR_NATS_INVALID_URL     Error = "Invalid NATS server URL"
	ERROR_NATS_TOPIC_NOT_FOUND Error = "topic '{topic}' was not found."
	ERROR_NATS_TOPIC_PUBLISH   Error = "Failed to publish To topic"
	ERROR_NATS_TOPIC_SUBSCRIBE Error = "Failed to subscribe To topic"
	ERROR_NATS_DISCONNECT      Error = "Error while disconnecting from NATS server"

	// General Status Errors
	ERROR_INVALID_ACTION Error = "Invalid action."
	ERROR_UNKNOWN        Error = "Unknown error."
)
