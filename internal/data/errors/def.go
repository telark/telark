package errors

type Error string

const (
	ERROR_MARSHALL_PAYLOAD Error = "Error marshaling JSON payload"
	ERROR_SEND_REQUEST     Error = "Error on sending request"
	ERROR_READ_RESPONSE    Error = "Error on reading response body"
	ERROR_DECODE_RESPONSE  Error = "Error on decoding JSON response"
	ERROR_UNKNOWN          Error = "Unknown Error"
)
