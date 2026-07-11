package templates

import (
	"fmt"

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
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprNewImages, opAnyIn, patterns),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockImageTypes,
		TemplateCode: codeBlockImageTypes,
		RuleName:     templateBlockImageTypes,
		Kinds:        policies.WorkloadKinds,
		Ops:          opsCreateUpdate,
		Message:      fmt.Sprintf(msgBlockImagePatterns, meta.PlanName),
		Deny:         deny,
	}), nil
}

func init() {
	policies.Register(blockImageTypes{})
}
