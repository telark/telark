package shared

type (
	Status  string
	Action  string
	Unified struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	OperationTime struct {
		OnFullDate string `json:"onFullDate"`
		OnTimeAgo  string `json:"onTimeAgo"`
	}
	ConditionStatus string
	Condition       struct {
		Type               string          `json:"type"`
		Status             ConditionStatus `json:"status"`
		Reason             string          `json:"reason,omitempty"`
		Message            string          `json:"message,omitempty"`
		LastTransitionTime string          `json:"lastTransitionTime,omitempty"`
		ObservedGeneration int64           `json:"observedGeneration,omitempty"`
	}
)
