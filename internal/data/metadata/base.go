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

type Group string

const (
	Erpi Group = "erpi.plsyro"
	Caps Group = "caps.plsyro"
)

type Version string

const (
	alpha1 Version = "v1alpha1"
	alpha2 Version = "v1alpha2"
)

func (metadata *Metadata) GetApiVersion() string {
	return fmt.Sprintf("%s/%s", metadata.BaseGroup, metadata.Version)
}
