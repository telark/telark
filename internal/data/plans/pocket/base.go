package pocket

import (
	"fmt"

	"github.com/plsyro/common-pkg/v2/data/common"
)

type PocketPlan struct {
	PlanGlobalData PlanGlobalData `json:"planGlobalData"`
	PlanCustomData PlanCustomData `json:"planCustomData"`
}

type PlanGlobalData struct {
	Name         string               `json:"name"`
	CreationTime common.OperationTime `json:"creationTime"`
	UpdateTime   common.OperationTime `json:"updateTime"`
}

type PlanCustomData struct {
	Status Status `json:"status"`
	Coins  Coins  `json:"coins"`
}

type Status struct {
	FirstCaptured string `json:"firstCaptured"`
	Current       string `json:"current"`
}

type Coins struct {
	Dropped int `json:"dropped"`
}

func (p *PlanGlobalData) GetPlanName() string {
	return fmt.Sprintf("%s-plan", p.Name)
}
