package grouper

import "github.com/plsyro/scopes-pkg/scopes/common"

type ResourceAsGrouper struct {
	Fasid common.Fasid `json:"fasid"`
	Cacid Cacid        `json:"cacid"`
}

type Cacid struct {
	Status    string         `json:"status"`
	History   []HistoryEvent `json:"history,omitempty"`
	Workloads []string       `json:"workloads"`
	Bridges   []string       `json:"bridges"`
	Customs   []string       `json:"customs"`
}

type HistoryEvent struct {
	Event        string `json:"event"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}
