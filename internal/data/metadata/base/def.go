package base

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
type Version string

const (
	ERPI  Group = "erpi.plsyro"
	FEATS Group = "feats.plsyro"
)

const (
	ALPHA_1 Version = "v1alpha1"
	ALPHA_2 Version = "v1alpha2"
)

func (metadata *Metadata) GetApiVersion() string {
	return fmt.Sprintf("%s/%s", metadata.BaseGroup, metadata.Version)
}
