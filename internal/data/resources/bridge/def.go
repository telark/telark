package bridge

import (
	resourceshared "github.com/plsyro/data/resources/shared"
	globalshared "github.com/plsyro/data/shared"
)

type BridgeAsResource struct {
	Fasid  resourceshared.Fasid  `json:"fasid"`
	Cacid  Cacid                 `json:"cacid"`
	Config resourceshared.Config `json:"config"`
}

type Cacid struct {
	Status    string                 `json:"status"`
	Type      string                 `json:"type"`
	Ports     []Port                 `json:"ports"`
	Selectors []globalshared.Unified `json:"selectors"`
	Workloads []Workload             `json:"workloads"`
}

type Workload struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	IsSameGrouper bool                   `json:"isSameGrouper"`
	MatchedLabels []globalshared.Unified `json:"matchedLabels"`
}

type Port struct {
	Source int `json:"source"`
	Target int `json:"target"`
}
