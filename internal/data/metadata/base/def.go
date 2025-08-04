package base

import "fmt"

type Metadata struct {
	BaseGroup string `json:"baseGroup"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	Plural    string `json:"plural"`
	Namespace string `json:"namespace"`
}

type (
	Group   string
	Version string
)

const (
	Erpi   Group   = "erpi.plsyro"
	Feats  Group   = "feats.plsyro"
	Alpha1 Version = "v1alpha1"
	Alpha2 Version = "v1alpha2"
)

func (metadata *Metadata) GetApiVersion() string {
	return fmt.Sprintf("%s/%s", metadata.BaseGroup, metadata.Version)
}
