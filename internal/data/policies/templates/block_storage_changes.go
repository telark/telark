package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockStorageChanges = "block-storage-changes"
	rulePVCMutation             = "block-pvc-mutation"
	ruleWorkloadVolumeChanges   = "block-workload-volume-changes"
)

type blockStorageChanges struct{}

func (blockStorageChanges) TemplateID() string { return templateBlockStorageChanges }

func (blockStorageChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockStorageChanges, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             rulePVCMutation,
			MatchResources:   policies.MatchAllAny(kindsPVC, opsUpdateDelete, scope.ApplicationIDs),
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockPVCMutation, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opIn, opsUpdateDelete),
				}),
			},
		},
		{
			Name:             ruleWorkloadVolumeChanges,
			MatchResources:   policies.MatchAllAny(policies.WorkloadKinds, opsUpdate, scope.ApplicationIDs),
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockVolumeChanges, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprNewVolumes, opNotEquals, exprOldVolumes),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockStorageChanges{})
}
