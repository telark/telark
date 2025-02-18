package errors

type Error string

const (
	// Rest
	ERROR_REST_MARSHALL_PAYLOAD Error = "Error On Marshaling JSON payload"
	ERROR_REST_SEND_REQUEST     Error = "Error On Sending Request"
	ERROR_REST_READ_RESPONSE    Error = "Error On Reading Response Body"
	ERROR_REST_DECODE_RESPONSE  Error = "Error on Decoding JSON Response"
	ERROR_REST_PARSE_BODY       Error = "Error On Parsing Request"
	ERROR_REST_MISSING_PARAM    Error = "{param} Parameter Is required In the URL Path"
	ERROR_REST_MISSING_BODY     Error = "{body} Is required For The {action} Action"

	// Clients
	ERROR_CLIENT_KUBE Error = "Failed To Get Kubernetes Client"

	// Resources
	ERROR_CREATE_RESOURCE              Error = "Error On Creating Resource"
	ERROR_GET_RESOURCE                 Error = "Error On Fetching Resource"
	ERROR_LIST_RESOURCES               Error = "Error On Listing Resources"
	ERROR_UPDATE_RESOURCE              Error = "Error On Updating Resource"
	ERROR_UPDATE_RESOURCE_WITH_HISTORY Error = "Error On Updating Resource with History"
	ERROR_UPDATE_RESOURCE_SYNC         Error = "Error On Updating Resource Sync Settings"
	ERROR_FILTER_RESOURCES             Error = "Error On Filtering Resources"
	ERROR_FILTER_RESOURCE              Error = "Error On Filtering Resource"
	ERROR_CHECK_RESOURCE_EXISTANCE     Error = "Error On Checking Resource Existence"
	ERROR_RESOURCE_EXISTS              Error = "Resource Already Exists"
	ERROR_RESOURCE_NOT_FOUND           Error = "Resource Cannot Be Found"

	// Admissions
	ERROR_CREATE_VALIDATING_ADMISSION Error = "Error On Creating Admission Validating Webhook "
	ERROR_CREATE_MUTATING_ADMISSION   Error = "Error On Creating Admission Mutating Webhook"
	ERROR_GET_VALIDATING_ADMISSION    Error = "Error On Fetching Admission Validating Webhook"
	ERROR_GET_MUTATING_ADMISSION      Error = "Error On Fetching Admission Mutating Webhook"
	ERROR_FILTER_ADMISSION            Error = "Error On Filtering Admission Webhook"

	// Common
	ERROR_INVALID_ACTION Error = "Invalid Action"
	ERROR_UNKNOWN        Error = "Unknown Error"
)
