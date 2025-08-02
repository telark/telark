package bridge

import (
	baseCommon "github.com/plsyro/data-pkg/common"
	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
)

type BridgeAsResource struct {
	Fasid  resourcesCommon.Fasid  `json:"fasid"`
	Cacid  Cacid                  `json:"cacid"`
	Config resourcesCommon.Config `json:"config"`
}

type Cacid struct {
	Status    string               `json:"status"`
	Type      string               `json:"type"`
	Ports     []Port               `json:"ports"`
	Selectors []baseCommon.Unified `json:"selectors"`
	Workloads []Workload           `json:"workloads"`
}

type Workload struct {
	Name          string               `json:"name"`
	Type          string               `json:"type"`
	IsSameGrouper bool                 `json:"isSameGrouper"`
	MatchedLabels []baseCommon.Unified `json:"matchedLabels"`
}

type Port struct {
	Source int `json:"source"`
	Target int `json:"target"`
}
