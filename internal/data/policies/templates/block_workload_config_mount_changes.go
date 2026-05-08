package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockMountChanges = "block-workload-config-mount-changes"

type blockMountChanges struct{}

func (blockMountChanges) TemplateID() string { return templateBlockMountChanges }

func (blockMountChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockMountChanges, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:           templateBlockMountChanges,
			MatchResources: policies.MatchAllAny(policies.WorkloadKinds, opsUpdate, scope.ApplicationIDs),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockConfigMountChng, meta.PlanName),
				Deny: &kyvernov1.Deny{
					RawAnyAllConditions: &kyvernov1.ConditionsWrapper{
						Conditions: kyvernov1.AnyAllConditions{
							AnyConditions: []kyvernov1.Condition{
								policies.MakeCondition(exprNewCMVolumes, opNotEquals, exprOldCMVolumes),
								policies.MakeCondition(exprNewSecretVolumes, opNotEquals, exprOldSecretVolumes),
								policies.MakeCondition(exprNewEnvFrom, opNotEquals, exprOldEnvFrom),
							},
						},
					},
				},
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockMountChanges{})
}
