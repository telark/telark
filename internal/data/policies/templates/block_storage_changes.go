package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockStorageChanges = "block-storage-changes"
	codeBlockStorageChanges     = "bsc"
	rulePVCMutation             = "block-pvc-mutation"
	ruleWorkloadVolumeChanges   = "block-workload-volume-changes"
)

type blockStorageChanges struct{}

func (blockStorageChanges) TemplateID() string   { return templateBlockStorageChanges }
func (blockStorageChanges) TemplateCode() string { return codeBlockStorageChanges }
func (blockStorageChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	rules := make([]kyvernov1.Rule, 0, 2)

	if pvcMatch, ok := policies.BuildMatch(scope, kindsPVC, opsUpdateDelete); ok {
		rules = append(rules, kyvernov1.Rule{
			Name:             rulePVCMutation,
			MatchResources:   pvcMatch,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockPVCMutation, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opIn, opsUpdateDelete),
				}),
			},
		})
	}

	if volMatch, ok := policies.BuildMatch(scope, policies.WorkloadKinds, opsUpdate); ok {
		rules = append(rules, kyvernov1.Rule{
			Name:             ruleWorkloadVolumeChanges,
			MatchResources:   volMatch,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockVolumeChanges, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprNewVolumes, opNotEquals, exprOldVolumes),
				}),
			},
		})
	}

	if len(rules) == 0 {
		return nil, nil
	}

	pol := policies.PolicyShell(meta, templateBlockStorageChanges, codeBlockStorageChanges, scope)
	pol.Spec.Rules = rules
	return pol, nil
}

func init() {
	policies.Register(blockStorageChanges{})
}
