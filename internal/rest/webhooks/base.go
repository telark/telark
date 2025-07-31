package webhooks

type Webhook string

const (
	VALIDATE_WITH_UPDATE_ALLOW Webhook = "validate-with-update-allow"
	VALIDATE_WITH_UPDATE_DENY  Webhook = "validate-with-update-deny"
)
