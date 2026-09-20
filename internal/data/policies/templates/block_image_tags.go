package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockImageTags = "block-image-tags"
	codeBlockImageTags     = "bitg"
	paramTags              = "tags"
)

type blockImageTags struct{}

func (blockImageTags) TemplateID() string   { return templateBlockImageTags }
func (blockImageTags) TemplateCode() string { return codeBlockImageTags }

func (blockImageTags) Render(meta policies.RenderMeta, scope policies.ScopeSpec, params map[string]any) (*kyvernov1.Policy, error) {
	tags, err := paramStringSlice(params, paramTags)
	if err != nil {
		return nil, err
	}
	// Matching the raw image string missed `nginx`, which Kubernetes resolves to `nginx:latest`.
	// Kyverno's parsed image info defaults an untagged reference to `latest` and also knows a
	// CronJob's deeper pod-template path, so one rule covers every workload kind.
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprImageTags, opAnyIn, tags),
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
