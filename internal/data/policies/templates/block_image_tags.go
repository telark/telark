package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockImageTags = "block-image-tags"
	codeBlockImageTags     = "bitg"
	paramTags              = "tags"
	imageTagPrefix         = "*:"
)

type blockImageTags struct{}

func (blockImageTags) TemplateID() string   { return templateBlockImageTags }
func (blockImageTags) TemplateCode() string { return codeBlockImageTags }

func (blockImageTags) Render(meta policies.RenderMeta, scope policies.ScopeSpec, params map[string]any) (*kyvernov1.Policy, error) {
	tags, err := paramStringSlice(params, paramTags)
	if err != nil {
		return nil, err
	}
	suffixed := make([]string, len(tags))
	for i, tag := range tags {
		suffixed[i] = imageTagPrefix + tag
	}
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprNewImages, opAnyIn, suffixed),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockImageTags,
		TemplateCode: codeBlockImageTags,
		RuleName:     templateBlockImageTags,
		Kinds:        policies.WorkloadKinds,
		Ops:          opsCreateUpdate,
		Message:      fmt.Sprintf(msgBlockImageTags, meta.PlanName),
		Deny:         deny,
	}), nil
}

func init() {
	policies.Register(blockImageTags{})
}
