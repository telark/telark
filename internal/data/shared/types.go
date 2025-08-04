package shared

type (
	Status      string
	WebhookType string
	Action      string
	Unified     struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	OperationTime struct {
		OnFullDate string `json:"onFullDate"`
		OnTimeAgo  string `json:"onTimeAgo"`
	}
)
