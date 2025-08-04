package grouper

import resourceShared "github.com/plsyro/data-pkg/resources/shared"

type GrouperAsResource struct {
	Fasid  resourceShared.Fasid  `json:"fasid"`
	Cacid  Cacid                 `json:"cacid"`
	Config resourceShared.Config `json:"config"`
}

type Cacid struct {
	Status    string            `json:"status"`
	Workloads []ManagedResource `json:"workloads"`
	Bridges   []ManagedResource `json:"bridges"`
}

type ManagedResource struct {
	Name       string `json:"name"`
	SourceName string `json:"sourceName"`
	Type       string `json:"type"`
	SourceType string `json:"sourceType"`
	LastSync   string `json:"lastSync"`
	Status     string `json:"status"`
}
