package templates

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/policies"
)

const (
	templateBlockStorageChanges = "block-storage-changes"
	codeBlockStorageChanges     = "bsc"
	rulePVCMutation             = "block-pvc-mutation"
	ruleWorkloadVolumeChanges   = "block-workload-volume-changes"
	storageRuleCapacity         = 2
)

type blockStorageChanges struct{}

func (blockStorageChanges) TemplateID() string   { return templateBlockStorageChanges }
func (blockStorageChanges) TemplateCode() string { return codeBlockStorageChanges }
func (blockStorageChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	rules := make([]kyvernov1.Rule, constants.DefaultInitValue, storageRuleCapacity)

	if match, ok := pvcMatch(scope); ok {
		rules = append(rules, kyvernov1.Rule{
			Name:           rulePVCMutation,
			MatchResources: match,
			// The PV controller completes a bind and the scheduler writes selected-node as PVC
			// updates; denying those leaves a Pending claim unable to ever bind.
			ExcludeResources: policies.ExcludeControllerWrites(),
			Validation: policies.Validation(
				blockMessage(meta, msgBlockPVCMutation),
				policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opIn, opsCreateUpdateDelete),
				}),
			),
		})
	}

	rules = append(rules, policies.PodSpecRules(scope, policies.PodSpecRuleSpec{
		RuleName: ruleWorkloadVolumeChanges,
		Ops:      opsUpdate,
		Message:  blockMessage(meta, msgBlockVolumeChanges),
		Deny: func(podSpecPath string) *kyvernov1.Deny {
			return policies.DenyWithConditions([]kyvernov1.Condition{
				policies.MakeCondition(
					newExpr(exprVolumesFmt, podSpecPath),
					opNotEquals,
					oldExpr(exprVolumesFmt, podSpecPath),
				),
			})
		},
	})...)

	if len(rules) == constants.DefaultInitValue {
		return nil, nil
	}

	pol := policies.PolicyShell(meta, templateBlockStorageChanges, codeBlockStorageChanges, scope)
	pol.Spec.Rules = rules
	return pol, nil
}

// An application never owns its PVCs, so the name-filtered match rendered no PVC rule at all;
// application scope matches the claim names its workloads mount instead.
// Both scopes cover CREATE so the rule means what the template description promises.
func pvcMatch(scope policies.ScopeSpec) (kyvernov1.MatchResources, bool) {
	if len(scope.ApplicationRefs) == constants.DefaultInitValue {
		return policies.BuildMatch(scope, kindsPVC, opsCreateUpdateDelete)
	}
	return policies.MatchNamed(kindsPVC, opsCreateUpdateDelete, scope.VolumeClaims)
}

func init() {
	policies.Register(blockStorageChanges{})
}
