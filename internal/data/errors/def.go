package errors

type Error string

const (
	ERROR_MARSHALL_PAYLOAD             Error = "Error marshaling JSON payload"
	ERROR_SEND_REQUEST                 Error = "Error on sending request"
	ERROR_READ_RESPONSE                Error = "Error on reading response body"
	ERROR_DECODE_RESPONSE              Error = "Error on decoding JSON response"
	ERROR_UNKNOWN                      Error = "Unknown Error"
	ERROR_KUBE_RESOURCE_CLIENT         Error = "Failed To Get Kubernetes Resource Client"
	ERROR_CREATE_RESOURCE              Error = "Error On Creating Resource"
	ERROR_GET_RESOURCE                 Error = "Error On Fetching Resource"
	ERROR_UPDATE_RESOURCE              Error = "Error On Updating Resource"
	ERROR_UPDATE_RESOURCE_WITH_HISTORY Error = "Error On Updating Resource with History"
	ERROR_RESOURCE_EXISTS              Error = "Resource Already Exists"
)
