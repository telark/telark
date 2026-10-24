package bootstrap

import "github.com/telark/telark/internal/rest/base"

// Served by the exporter, the only writer of the pin and the requests (TelarkConfig status).
const (
	GetBootstrap    base.Endpoint = "bootstrap"
	OpenRequest     base.Endpoint = "bootstrap/requests"
	ApproveRequest  base.Endpoint = "bootstrap/requests/{id}/approve"
	CancelRequest   base.Endpoint = "bootstrap/requests/{id}/cancel"
	CompleteRequest base.Endpoint = "bootstrap/requests/{id}/complete"
)

// The approval ceremony, served by auth: a passkey assertion with no sign-in, bound to the request and the action.
const (
	StartAssertion  base.Endpoint = "auth/bootstrap/requests/{id}/assert/start"
	FinishAssertion base.Endpoint = "auth/bootstrap/requests/{id}/assert/finish"
)
