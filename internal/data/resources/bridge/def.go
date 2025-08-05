package bridge

import (
	resourceShared "github.com/plsyro/data/resources/shared"
	globalShared "github.com/plsyro/data/shared"
)

type BridgeAsResource struct {
	Fasid  resourceShared.Fasid  `json:"fasid"`
	Cacid  Cacid                 `json:"cacid"`
	Config resourceShared.Config `json:"config"`
}

type Cacid struct {
	Status    string                 `json:"status"`
	Type      string                 `json:"type"`
	Ports     []Port                 `json:"ports"`
	Selectors []globalShared.Unified `json:"selectors"`
	Workloads []Workload             `json:"workloads"`
}

type Workload struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	IsSameGrouper bool                   `json:"isSameGrouper"`
	MatchedLabels []globalShared.Unified `json:"matchedLabels"`
}

type Port struct {
	Source int `json:"source"`
	Target int `json:"target"`
}
