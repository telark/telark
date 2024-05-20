package coin

import (
	"fmt"
	"strings"

	"github.com/plsyro/common-pkg/v2/data/common"
)

type CoinPlan struct {
	PlanGlobalData PlanGlobalData `json:"planGlobalData"`
	PlanCustomData PlanCustomData `json:"planCustomData"`
}

type PlanGlobalData struct {
	Name         string               `json:"name"`
	Pocket       string               `json:"pocket"`
	CreationTime common.OperationTime `json:"creationTime"`
	UpdateTime   common.OperationTime `json:"updateTime"`
}

type PlanCustomData struct {
	CoinControllerData CoinControllerData `json:"coinControllerData"`
	CoinServiceData    []CoinServiceData  `json:"coinServiceData"`
}

type CoinControllerData struct {
	Metadata                ControllerMetadata      `json:"metadata"`
	Kind                    string                  `json:"kind"`
	Strategy                string                  `json:"strategy"`
	Instances               ControllerInstances     `json:"instances"`
	Crates                  Crates                  `json:"crates"`
	Registry                string                  `json:"registry"`
	Protection              Protection              `json:"protection"`
	TrafficAttachmentPolicy TrafficAttachmentPolicy `json:"trafficAttachmentPolicy,omitempty"`
	Events                  []ControllerEvent       `json:"events"`
}

type ControllerMetadata struct {
	Name         string               `json:"name"`
	Pocket       string               `json:"pocket"`
	Annotations  []Unified            `json:"annotations"`
	Labels       ControllerLabels     `json:"labels"`
	CreationTime common.OperationTime `json:"creationTime"`
}

type Unified struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ControllerLabels struct {
	Global   []Unified `json:"global"`
	Selector []Unified `json:"selector"`
}

type ControllerInstances struct {
	Total     int       `json:"total"`
	Available int       `json:"available"`
	Names     []string  `json:"names"`
	Labels    []Unified `json:"labels"`
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

type Protection struct {
	IsSecConApplied bool `json:"isSecConApplied"`
}

type TrafficAttachmentPolicy string

const (
	OneToMany         TrafficAttachmentPolicy = "OneToMany"
	OneToOne          TrafficAttachmentPolicy = "OneToOne"
	NoServiceAssigned TrafficAttachmentPolicy = "No Service Assigned"
)

type ControllerEvent struct {
	Instance string  `json:"instance"`
	Events   []Event `json:"events"`
}

type Event struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Name struct {
	Name string `json:"name"`
}

type CoinServiceData struct {
	Name                  string    `json:"name"`
	Pocket                string    `json:"pocket"`
	Type                  string    `json:"type"`
	Ports                 []Port    `json:"ports"`
	ConnectedInPocket     bool      `json:"connectedInPocket"`
	Hosts                 []Host    `json:"hosts"`
	MatchedLabels         []Unified `json:"matchedLabels"`
	MatchedControllerPort bool      `json:"matchedControllerPort"`
}

type Port struct {
	SourcePort int `json:"sourcePort"`
	TargetPort int `json:"targetPort"`
}

type Host struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type CoinData struct {
	Pocket     string
	Controller string

	Name string
}

func GenerateCoinData(pocket string, controller string) *CoinData {
	coin := &CoinData{
		Pocket:     pocket,
		Controller: controller,
	}

	name := coin.CoinName()

	coin.Name = name

	return coin
}

func (c *CoinData) CoinName() string {
	return fmt.Sprintf("%s-%s-coin", strings.ToLower(c.Pocket), strings.ToLower(c.Controller))
}
