package shared

import globalshared "github.com/plsyro/data/shared"

type ComposedLabels struct {
	Global   []globalshared.Unified `json:"global"`
	Selector []globalshared.Unified `json:"selector"`
}

type Metadata struct {
	Annotations []globalshared.Unified `json:"annotations"`
	Labels      ComposedLabels         `json:"labels"`
}

type Instances struct {
	Total     int                    `json:"total"`
	Available int                    `json:"available"`
	Names     []string               `json:"names"`
	Labels    []globalshared.Unified `json:"labels"`
}

type Crates struct {
	Regular []Crate `json:"regular"`
	Init    []Crate `json:"init"`
}

type Crate struct {
	Name    string     `json:"name"`
	Order   int        `json:"order"`
	SubType CrateType  `json:"subType"`
	Image   CrateImage `json:"image"`
	Ports   []int      `json:"ports"`
}

type Usage struct {
	QoS       string   `json:"qos"`
	Resources Resource `json:"resources"`
	Available bool     `json:"available"`
	Timestamp string   `json:"timestamp"`
}

type Resource struct {
	TotalCPU         string             `json:"totalCpu"`
	TotalMemory      string             `json:"totalMemory"`
	UsagePerInstance []UsagePerInstance `json:"usagePerInstance"`
}

type UsagePerInstance struct {
	Name        string           `json:"name"`
	Containers  []ContainerUsage `json:"containers"`
	TotalCPU    string           `json:"totalCpu"`
	TotalMemory string           `json:"totalMemory"`
}

type ContainerUsage struct {
	Name   string `json:"name"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type CrateImage struct {
	Name       string `json:"name"`
	Tag        string `json:"tag"`
	PullPolicy string `json:"pullPolicy"`
	IsCurrent  bool   `json:"isCurrent"`
}

type Events struct {
	Instance string      `json:"instance"`
	Events   []EventItem `json:"events"`
}

type EventItem struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type (
	CrateType              string
	BridgeAttachmentPolicy string
	RegType                string
)

const (
	Standard          CrateType              = "Standard"
	SideCar           CrateType              = "SideCar"
	OneToMany         BridgeAttachmentPolicy = "Many Bridges Assigned"
	OneToOne          BridgeAttachmentPolicy = "One Bridge Assigned"
	NoServiceAssigned BridgeAttachmentPolicy = "No Bridge Assigned"
	Prv               RegType                = "Private"
	Pub               RegType                = "Public"
)

const (
	NoneStrategyType       = "None"
	ReadyConditionType     = "Ready"
	AvailableConditionType = "Available"
)
