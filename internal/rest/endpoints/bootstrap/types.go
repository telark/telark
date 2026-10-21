package bootstrap

import "github.com/telark/telark/internal/data/resources/telarkconfig"

type OpenBody struct {
	Email string `json:"email"`
}

type CancelBody struct {
	Reason string `json:"reason"`
}

// Revoke: break-glass's last pass while the account is frozen purges a passkey enrolled since the completion too.
type CompleteBody struct {
	Revoke bool `json:"revoke"`
}

// Enrolled: after a completed request, whether the account it concerns enrolled a passkey since.
type RequestResult struct {
	Request  telarkconfig.BootstrapRequest `json:"request"`
	Eligible bool                          `json:"eligible"`
	Enrolled bool                          `json:"enrolled"`
}

// GET bootstrap. NotifyAdmins: whether every Admin on ALL hears of the requests and reads them here.
type StatusResult struct {
	telarkconfig.BootstrapStatus
	NotifyAdmins bool `json:"notifyAdmins"`
}

// Action is approve or deny.
type AssertionBody struct {
	Action string `json:"action"`
}
