package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockMountChanges = "block-workload-config-mount-changes"
	codeBlockMountChanges     = "bwcm"
)

type blockMountChanges struct{}

func (blockMountChanges) TemplateID() string   { return templateBlockMountChanges }
func (blockMountChanges) TemplateCode() string { return codeBlockMountChanges }

func (blockMountChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	return policies.RenderPodSpecRulePolicy(meta, scope, policies.PodSpecRuleSpec{
		TemplateID:   templateBlockMountChanges,
		TemplateCode: codeBlockMountChanges,
		RuleName:     templateBlockMountChanges,
		Ops:          opsUpdate,
		Message:      fmt.Sprintf(msgBlockConfigMountChng, meta.PlanName),
		Deny:         mountChangesDeny,
	}), nil
}

func mountChangesDeny(podSpecPath string) *kyvernov1.Deny {
	return &kyvernov1.Deny{
		RawAnyAllConditions: &kyvernov1.ConditionsWrapper{
			Conditions: kyvernov1.AnyAllConditions{
				AnyConditions: []kyvernov1.Condition{
					mountCondition(exprCMVolumesFmt, podSpecPath),
					mountCondition(exprSecretVolumesFmt, podSpecPath),
					mountCondition(exprEnvFromFmt, podSpecPath),
					mountCondition(exprEnvRefsFmt, podSpecPath),
					mountCondition(exprVolumeMountsFmt, podSpecPath),
				},
			},
		},
	}
}

func mountCondition(format, podSpecPath string) kyvernov1.Condition {
	return policies.MakeCondition(newExpr(format, podSpecPath), opNotEquals, oldExpr(format, podSpecPath))
}

func init() {
	policies.Register(blockMountChanges{})
}
