package shared

import "net/http"

type KubernetesAPIData struct {
	Status  int
	Message string
	Data    any
	Error   error
}

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusInternalServerError = http.StatusInternalServerError
)
