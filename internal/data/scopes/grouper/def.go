package grouper

import "github.com/plsyro/scopes-pkg/scopes/common"

type ResourceAsGrouper struct {
	Fasid  common.Fasid  `json:"fasid"`
	Cacid  Cacid         `json:"cacid"`
	Config common.Config `json:"config"`
}

type Cacid struct {
	Status    string   `json:"status"`
	Workloads []string `json:"workloads"`
	Bridges   []string `json:"bridges"`
	Customs   []string `json:"customs"`
}
