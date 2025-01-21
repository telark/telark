package grouper

import (
	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
)

type GrouperAsResource struct {
	Fasid  resourcesCommon.Fasid  `json:"fasid"`
	Cacid  Cacid                  `json:"cacid"`
	Config resourcesCommon.Config `json:"config"`
}

type Cacid struct {
	Status    string            `json:"status"`
	Workloads []ManagedResource `json:"workloads"`
	Bridges   []ManagedResource `json:"bridges"`
	Customs   []ManagedResource `json:"customs"`
}

type ManagedResource struct {
	Name     string `json:"name"`
	LastSync string `json:"lastSync"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
}
