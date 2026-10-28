package templates

import (
	"fmt"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockImageTypes = "block-image-types"
	codeBlockImageTypes     = "bit"
	paramImagePatterns      = "imagePatterns"
)

type blockImageTypes struct{}

func (blockImageTypes) TemplateID() string   { return templateBlockImageTypes }
func (blockImageTypes) TemplateCode() string { return codeBlockImageTypes }

func (blockImageTypes) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	params map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	patterns, err := paramStringSlice(params, paramImagePatterns)
	if err != nil {
		return nil, err
	}
	return policies.RenderPodSpecRulePolicy(meta, scope, policies.PodSpecRuleSpec{
		TemplateID:   templateBlockImageTypes,
		TemplateCode: codeBlockImageTypes,
		Ops:          opsCreateUpdate,
		Message:      blockMessage(meta, msgBlockImagePatterns),
		Admit:        fmt.Sprintf(admitNoImageFmt, policies.VarContainers, policies.CELString(policies.GlobRegex(patterns))),
	}), nil
}

func init() {
	policies.Register(blockImageTypes{})
}
