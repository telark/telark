package bridge

import "github.com/plsyro/scopes-pkg/scopes/common"

type ResourceAsBridge struct {
	Fasid common.Fasid `json:"fasid"`
	Cacid Cacid        `json:"cacid"`
}

type Cacid struct {
	Type      string     `json:"type"`
	Ports     []Port     `json:"ports"`
	Workloads []Workload `json:"workloads"`
}

type Workload struct {
	Name          int              `json:"name"`
	Grouper       int              `json:"grouper"`
	Kind          int              `json:"kind"`
	IsSameGrouper int              `json:"isSameGrouper"`
	IsPortMatched bool             `json:"isPortMatched"`
	Hosts         []string         `json:"hosts"`
	MatchedLabels []common.Unified `json:"matchedLabels"`
}

type Port struct {
	Source int `json:"source"`
	Target int `json:"target"`
}
