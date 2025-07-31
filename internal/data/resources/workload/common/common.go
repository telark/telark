package common

import (
	"github.com/plsyro/data-pkg/common"
)

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
	Available int              `json:"available"`
	Names     []string         `json:"names"`
	Labels    []common.Unified `json:"labels"`
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
	STANDARD CrateType = "Standard"
	SIDE_CAR CrateType = "SideCar"
)

const (
	ONE_TO_MANY         BridgeAttachmentPolicy = "Many Bridges Assigned"
	ONE_TO_ONE          BridgeAttachmentPolicy = "One Bridge Assigned"
	NO_SERVICE_ASSIGNED BridgeAttachmentPolicy = "No Bridge Assigned"
)

const (
	PRV RegType = "Private"
	PUB RegType = "Public"
)

const (
	NONE_STRATEGY_TYPE       = "None"
	READY_CONDITION_TYPE     = "Ready"
	AVAILABLE_CONDITION_TYPE = "Available"
)
