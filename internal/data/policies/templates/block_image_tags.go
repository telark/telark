package templates

import (
	"fmt"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockImageTags = "block-image-tags"
	codeBlockImageTags     = "bitg"
	paramTags              = "tags"
)

type blockImageTags struct{}

func (blockImageTags) TemplateID() string   { return templateBlockImageTags }
func (blockImageTags) TemplateCode() string { return codeBlockImageTags }

func (blockImageTags) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	params map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	tags, err := paramStringSlice(params, paramTags)
	if err != nil {
		return nil, err
	}
	// Kyverno's image library reads an untagged `nginx` as `latest` (the raw string missed it); a
	// digest-pinned reference has no tag and is deliberately out of scope for a tag block list.
	images := fmt.Sprintf(exprImagesFmt, policies.VarContainers)
	return policies.RenderPodSpecRulePolicy(meta, scope, policies.PodSpecRuleSpec{
		TemplateID:   templateBlockImageTags,
		TemplateCode: codeBlockImageTags,
		Ops:          opsCreateUpdate,
		Message:      blockMessage(meta, msgBlockImageTags),
		Admit:        fmt.Sprintf(admitNoImageTagFmt, images, policies.CELList(tags)),
	}), nil
}

func init() {
	policies.Register(blockImageTags{})
}
