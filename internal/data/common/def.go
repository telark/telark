package common

type OperationTime struct {
	OnFullDate string `json:"onFullDate"`
	OnTimeAgo  string `json:"onTimeAgo"`
}

type Unified struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var (
	ManagedFields = []string{
		"apiVersion",
		"kind",
		"metadata.generation",
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.uid",
	}
)
