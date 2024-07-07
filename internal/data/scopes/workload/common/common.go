package workload

import "github.com/plsyro/scopes-pkg/scopes/common"

type Metadata struct {
	Annotations []common.Unified `json:"annotations"`
	Labels      ComposedLabels   `json:"labels"`
}

type ComposedLabels struct {
	Global   []common.Unified `json:"global"`
	Selector []common.Unified `json:"selector"`
}

type Instances struct {
	Total     int              `json:"total"`
	Kind      string           `json:"kind"`
	Available int              `json:"available"`
	Names     []string         `json:"names"`
	Labels    []common.Unified `json:"labels"`
}

type Crates struct {
	Regular []Crate `json:"regular"`
	Init    []Crate `json:"init"`
}

type Crate struct {
	Name      string        `json:"name"`
	Order     int           `json:"order"`
	SubType   CrateType     `json:"subType"`
	Resources CrateResource `json:"resources"`
	Image     CrateImage    `json:"image"`
	Ports     []int         `json:"ports"`
}

type CrateType string

const (
	Std CrateType = "Standard"
	Sc  CrateType = "SideCar"
)

type CrateResource struct {
	Qos    string `json:"qos"`
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type CrateImage struct {
	Name       string `json:"name"`
	Tag        string `json:"tag"`
	PullPolicy string `json:"pullPolicy"`
	IsCurrent  bool   `json:"isCurrent"`
}

type BridgeAttachmentPolicy string

const (
	OneToMany        BridgeAttachmentPolicy = "OneToMany"
	OneToOne         BridgeAttachmentPolicy = "OneToOne"
	NoBridgeAssigned BridgeAttachmentPolicy = "No Bridge Assigned"
)

type Events struct {
	Instance string      `json:"instance"`
	Events   []EventItem `json:"events"`
}

type EventItem struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
