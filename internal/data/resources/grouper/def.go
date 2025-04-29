package grouper

import "github.com/plsyro/data-pkg/resources/common"

type GrouperAsResource struct {
	Fasid  common.Fasid `json:"fasid"`
	Cacid  Cacid        `json:"cacid"`
	Config Config       `json:"config"`
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

type Config struct {
	History     []common.Record `json:"history"`
	Sync        common.Sync     `json:"sync"`
	Maintenance Maintenance     `json:"maintenance"`
}

type Maintenance struct {
	Status          string `json:"Status"`
	Name            string `json:"Name"`
	AttachedWebhook string `json:"attachedWebhook"`
}
