package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/policies"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

const (
	templateBlockStorageChanges = "block-storage-changes"
	codeBlockStorageChanges     = "bsc"
	varPVCRule                  = "pvcRule"
	varVolumeRule               = "volumeRule"
	admitNotPVCRule             = "!variables." + varPVCRule
	admitNotVolumeRule          = "!variables." + varVolumeRule
	storageRuleCapacity         = 2
)

type blockStorageChanges struct{}

func (blockStorageChanges) TemplateID() string   { return templateBlockStorageChanges }
func (blockStorageChanges) TemplateCode() string { return codeBlockStorageChanges }

// Two branches with their own message: the variables say which one a request fell in.
func (blockStorageChanges) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	var match []policies.ResourceFilter
	variables := make([]admissionregistrationv1.Variable, constants.DefaultInitValue, storageRuleCapacity)
	validations := make([]admissionregistrationv1.Validation, constants.DefaultInitValue, storageRuleCapacity)

	if pvc, ok := pvcMatch(scope); ok {
		// The PV controller completes a bind and the scheduler writes selected-node as PVC
		// updates; denying those leaves a Pending claim unable to ever bind.
		pvc = policies.ExceptControllers(pvc)
		match = append(match, pvc...)
		variables = append(variables, admissionregistrationv1.Variable{Name: varPVCRule, Expression: policies.AnyOf(pvc)})
		validations = append(validations, policies.Validation(admitNotPVCRule, blockMessage(meta, msgBlockPVCMutation)))
	}

	if workloads, ok := policies.BuildMatch(scope, policies.WorkloadKinds, opsUpdate); ok {
		match = append(match, workloads...)
		variables = append(variables, admissionregistrationv1.Variable{Name: varVolumeRule, Expression: policies.AnyOf(workloads)})
		variables = append(variables, policies.PodSpecVariables()...)
		admit := admitNotVolumeRule + celOr + same(exprVolumesFmt, policies.VarPodSpec, policies.VarOldPodSpec)
		validations = append(validations, policies.Validation(admit, blockMessage(meta, msgBlockVolumeChanges)))
	}

	if len(match) == constants.DefaultInitValue {
		return nil, nil
	}
	return policies.NewPolicy(meta, scope, policies.PolicySpec{
		TemplateID:   templateBlockStorageChanges,
		TemplateCode: codeBlockStorageChanges,
		Match:        match,
		Variables:    variables,
		Validations:  validations,
	}), nil
}

// An application never owns its PVCs, so the name-filtered match rendered no PVC rule at all;
// application scope matches the claim names its workloads mount instead.
// Both scopes cover CREATE so the rule means what the template description promises.
func pvcMatch(scope policies.ScopeSpec) ([]policies.ResourceFilter, bool) {
	if len(scope.ApplicationRefs) == constants.DefaultInitValue {
		return policies.BuildMatch(scope, kindsPVC, opsCreateUpdateDelete)
	}
	return policies.MatchNamed(kindsPVC, opsCreateUpdateDelete, scope.VolumeClaims)
}

func init() {
	policies.Register(blockStorageChanges{})
}
