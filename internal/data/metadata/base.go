package metadata

import (
	"fmt"
)

type Metadata struct {
	BaseGroup string `json:"baseGroup"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	Plural    string `json:"plural"`
	Namespace string `json:"namespace"`
}

type Version string

const (
	alpha1 Version = "v1alpha1"
	alpha2 Version = "v1alpha2"
	V1     Version = "v1"
)

type Group string

const (
	Plan     Group   = "plsyro.plan"
	Insights Version = "plsyro.insights"
)

func (metadata *Metadata) GetApiVersion() string {
	return fmt.Sprintf("%s/%s", metadata.BaseGroup, metadata.Version)
}
