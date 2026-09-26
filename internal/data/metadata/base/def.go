package base

import "fmt"

type Metadata struct {
	BaseGroup string `json:"baseGroup"`
	Kind      string `json:"kind"`
	Version   string `json:"version"`
	Plural    string `json:"plural"`
	Namespace string `json:"namespace"`
	// View key to status key; an empty status key projects the whole status object.
	StatusFields map[string]string `json:"statusFields,omitempty"`
	// Object name of a kind that holds exactly one object in the namespace.
	Singleton string `json:"singleton,omitempty"`
}

const (
	Group    = "telark.io"
	V1Alpha1 = "v1alpha1"
)

func (metadata *Metadata) GetAPIVersion() string {
	return fmt.Sprintf("%s/%s", metadata.BaseGroup, metadata.Version)
}
