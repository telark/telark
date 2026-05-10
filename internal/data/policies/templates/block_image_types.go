package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockImageTypes = "block-image-types"
	codeBlockImageTypes     = "bit"
	paramImagePatterns      = "imagePatterns"
)

type blockImageTypes struct{}

func (blockImageTypes) TemplateID() string   { return templateBlockImageTypes }
func (blockImageTypes) TemplateCode() string { return codeBlockImageTypes }
func (blockImageTypes) Render(meta policies.RenderMeta, scope policies.ScopeSpec, params map[string]any) (*kyvernov1.Policy, error) {
	patterns, err := paramStringSlice(params, paramImagePatterns)
	if err != nil {
		return nil, err
	}

	match, ok := policies.BuildMatch(scope, policies.WorkloadKinds, opsCreateUpdate)
	if !ok {
		return nil, nil
	}

	pol := policies.PolicyShell(meta, templateBlockImageTypes, codeBlockImageTypes, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockImageTypes,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockImagePatterns, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprNewImages, opAnyIn, patterns),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockImageTypes{})
}
