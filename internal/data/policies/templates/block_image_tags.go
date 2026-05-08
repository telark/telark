package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockImageTags = "block-image-tags"
	paramTags              = "tags"
	imageTagPrefix         = "*:"
)

type blockImageTags struct{}

func (blockImageTags) TemplateID() string { return templateBlockImageTags }

func (blockImageTags) Render(meta policies.RenderMeta, scope policies.ScopeSpec, params map[string]any) (*kyvernov1.Policy, error) {
	tags, err := paramStringSlice(params, paramTags)
	if err != nil {
		return nil, err
	}

	suffixed := make([]string, len(tags))
	for i, tag := range tags {
		suffixed[i] = imageTagPrefix + tag
	}

	pol := policies.PolicyShell(meta, templateBlockImageTags, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:           templateBlockImageTags,
			MatchResources: policies.MatchAllAny(policies.WorkloadKinds, opsCreateUpdate, scope.ApplicationIDs),
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
