package health

import (
	"github.com/telark/data/plans"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	CheckTimeoutSeconds   = 15
	kyvernoConditionReady = "Ready"
	kyvernoStatusTrue     = "True"
	kyvernoActionEnforce  = "Enforce"
	kyvernoActionAudit    = "Audit"
)

type Result struct {
	Health     string
	Detail     []plans.ProtectionPlanHealthDetail
	Policies   []planseps.ProtectionPlanPolicyStatus
	Missing    []string
	Unexpected []string
}

type policySnapshot struct {
	namespace     string
	ready         bool
	failureAction string
}

type healthFlags struct {
	notReady bool
	drifted  bool
}
