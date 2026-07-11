package shared

import (
	"net/http"

	"github.com/telark/exporter/internal/constants"
)

type ResponseCapture struct {
	http.ResponseWriter
	status int
}

func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{ResponseWriter: w}
}

func (rc *ResponseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *ResponseCapture) Write(b []byte) (int, error) {
	if rc.status == constants.DefaultInitValue {
		rc.status = http.StatusOK
	}
	return rc.ResponseWriter.Write(b)
}

func (rc *ResponseCapture) Status() int {
	if rc.status == constants.DefaultInitValue {
		return http.StatusOK
	}
	return rc.status
}
