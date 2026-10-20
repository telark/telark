package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

const (
	templateBlockUpdate = "block-update"
	codeBlockUpdate     = "bu"
)

type blockUpdate struct{}

func (blockUpdate) TemplateID() string   { return templateBlockUpdate }
func (blockUpdate) TemplateCode() string { return codeBlockUpdate }

func (blockUpdate) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	main, ok := policies.BuildMatch(scope, kindsWildcard, opsUpdate)
	if !ok {
		return nil, nil
	}
	// The wildcard reaches the PVC a controller is binding and the ReplicaSet a rollout is
	// resizing, so those writes stay exempt.
	match := policies.ExceptControllers(main)
	// The scale branch must NOT exempt the controller identities: the HPA controller shares them
	// and `kubectl autoscale` went through.
	if scale, ok := policies.BuildMatch(scope, policies.ScaleSubresourceKinds, opsUpdate); ok {
		match = append(match, scale...)
	}
	return policies.NewPolicy(meta, scope, policies.PolicySpec{
		TemplateID:   templateBlockUpdate,
		TemplateCode: codeBlockUpdate,
		Match:        match,
		Validations:  []admissionregistrationv1.Validation{policies.Validation(admitNever, blockMessage(meta, msgBlockUpdate))},
	}), nil
}

func init() {
	policies.Register(blockUpdate{})
}
