package bootstrap

import "github.com/telark/telark/internal/data/resources/telarkconfig"

type OpenBody struct {
	Email string `json:"email"`
}

type CancelBody struct {
	Reason string `json:"reason"`
}

// A recovery's passes: none set ends the account's sessions; Revoke, the last pass before the link, also deletes the
// passkeys saved after the 72-hour mark; Keep, the credential id the link registered, completes it.
type CompleteBody struct {
	Revoke bool   `json:"revoke,omitempty"`
	Keep   string `json:"keep,omitempty"`
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
