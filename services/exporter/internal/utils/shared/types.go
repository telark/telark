package shared

import "net/http"

type (
	ResponseCapture struct {
		http.ResponseWriter
		status int
	}

	UpstreamError struct {
		Status int
		Err    error
	}
)
