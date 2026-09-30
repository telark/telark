package templates

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
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
	return policies.RenderPodSpecRulePolicy(meta, scope, policies.PodSpecRuleSpec{
		TemplateID:   templateBlockImageTypes,
		TemplateCode: codeBlockImageTypes,
		RuleName:     templateBlockImageTypes,
		Ops:          opsCreateUpdate,
		Message:      blockMessage(meta, msgBlockImagePatterns),
		Deny: func(podSpecPath string) *kyvernov1.Deny {
			return policies.DenyWithConditions([]kyvernov1.Condition{
				policies.MakeCondition(newExpr(exprImagesFmt, podSpecPath), opAnyIn, patterns),
			})
		},
	}), nil
}

func init() {
	policies.Register(blockImageTypes{})
}
