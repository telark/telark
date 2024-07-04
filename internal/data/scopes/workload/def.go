package scopes

import (
	"fmt"
	"strings"

	"github.com/plsyro/scopes-pkg/scopes/common"
)

type ResourceAsWorkload struct {
	Fasid common.Fasid `json:"fasid"`
	Cacid Cacid        `json:"cacid"`
}

type Cacid struct {
	Metadata               ControllerMetadata     `json:"metadata"`
	Kind                   string                 `json:"kind"`
	Strategy               string                 `json:"strategy"`
	Instances              ControllerInstances    `json:"instances"`
	Crates                 Crates                 `json:"crates"`
	Registry               string                 `json:"registry"`
	BridgeAttachmentPolicy BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy,omitempty"`
	Events                 []ControllerEvent      `json:"events"`
}

type ControllerMetadata struct {
	Annotations []common.Unified `json:"annotations"`
	Labels      ComposedLabels   `json:"labels"`
}

type ComposedLabels struct {
	Global   []common.Unified `json:"global"`
	Selector []common.Unified `json:"selector"`
}

type ControllerInstances struct {
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

type BridgeAttachmentPolicy string

const (
	OneToMany        BridgeAttachmentPolicy = "OneToMany"
	OneToOne         BridgeAttachmentPolicy = "OneToOne"
	NoBridgeAssigned BridgeAttachmentPolicy = "No Bridge Assigned"
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
