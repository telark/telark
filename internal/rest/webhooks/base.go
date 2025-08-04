package webhooks

type Webhook string

const (
	ValidateWithUpdateAllow Webhook = "validate-with-update-allow"
	ValidateWithUpdateDeny  Webhook = "validate-with-update-deny"
)
