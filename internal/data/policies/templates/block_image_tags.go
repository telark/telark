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

	match, ok := policies.BuildMatch(scope, policies.WorkloadKinds, opsCreateUpdate)
	if !ok {
		return nil, nil
	}

	pol := policies.PolicyShell(meta, templateBlockImageTags, codeBlockImageTags, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockImageTags,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockImageTags, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprNewImages, opAnyIn, suffixed),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockImageTags{})
}
