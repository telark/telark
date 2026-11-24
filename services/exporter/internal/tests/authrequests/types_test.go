package authrequests

import "github.com/telark/telark/internal/data/resources/authrequest"

type pendingData struct {
	Items []authrequest.AuthRequest `json:"items"`
}

type pendingEnvelope struct {
	Message string      `json:"message"`
	Data    pendingData `json:"data"`
}
