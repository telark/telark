package bridge

import "github.com/plsyro/scopes-pkg/scopes/common"

type ResourceAsBridge struct {
	Fasid common.Fasid `json:"fasid"`
	Cacid Cacid        `json:"cacid"`
}

type Cacid struct {
	Type      string           `json:"type"`
	Ports     []Port           `json:"ports"`
	Selectors []common.Unified `json:"selectors"`
	Workloads []Workload       `json:"workloads"`
}

type Workload struct {
	Name          string           `json:"name"`
	Kind          string           `json:"kind"`
	IsSameGrouper bool             `json:"isSameGrouper"`
	MatchedLabels []common.Unified `json:"matchedLabels"`
}

type Port struct {
	Source int `json:"source"`
	Target int `json:"target"`
}
