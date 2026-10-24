package policies

import (
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/sdk/extensions/cel/libs/image"
	"github.com/telark/telark/internal/data/plans"
	datapolicies "github.com/telark/telark/internal/data/policies"
	_ "github.com/telark/telark/internal/data/policies/templates" // registers every renderer
	"github.com/telark/telark/services/discovery/internal/constants"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

const (
	acceptNamespace = "prod"
	acceptApp       = "wa1"
	acceptPlanID    = "pp-abc-1234-5678"
	acceptPlanName  = "freeze"
	kindDeployment  = "Deployment"
	lineBreaks      = "\r\n"
)

// Params for the two templates that take them; the rest render without any.
var acceptParams = map[string]map[string]any{
	"block-image-types": {"imagePatterns": []string{"*nginx*", "registry.example.com/?ebug:*"}},
	"block-image-tags":  {"tags": []string{"latest", "dev"}},
}

// Kyverno's vpol compiler version, which picks the CEL library set it compiles against.
var vpolCompilerVersion = version.MajorMinor(1, 0)

func acceptScopes() map[string]datapolicies.ScopeSpec {
	kinds := []string{"Deployment", "StatefulSet", "ConfigMap", "Secret", "CronJob"}
	refs := make([]datapolicies.ApplicationResourceRef, constants.DefaultInitValue, len(kinds))
	for _, kind := range kinds {
		refs = append(refs, datapolicies.ApplicationResourceRef{
			Kind: kind, Name: acceptApp, Namespace: acceptNamespace,
		})
	}
	return map[string]datapolicies.ScopeSpec{
		plans.ScopeTypeNamespaces: {Namespace: acceptNamespace},
		plans.ScopeTypeApplications: {
			Namespace:       acceptNamespace,
			ApplicationRefs: []string{acceptApp},
			AppResources:    refs,
			VolumeClaims:    []string{"pvc-a", "data-wa1-*"},
		},
	}
}

type compiledPolicy struct {
	policy          *policiesv1.NamespacedValidatingPolicy
	matchConditions []cel.Program
	variables       map[string]cel.Program
	validations     []compiler.Validation
}

// The environment of Kyverno 1.19's vpol compiler (createBaseVpolEnv). Kyverno's own compiler and
// engine packages can't build here: their legacy engine needs Kyverno's Pod Security fork, a
// replace this module can't carry. Of Kyverno's libraries the policies call only the image one.
func vpolEnv(t *testing.T) (*cel.Env, *compiler.VariablesProvider) {
	t.Helper()
	opts := append(
		compiler.DefaultEnvOptionsWithCompat(),
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)
	base := environment.MustBaseEnvSet(vpolCompilerVersion)
	stored, err := base.Env(environment.StoredExpressions)
	if err != nil {
		t.Fatalf("base env: %v", err)
	}
	variables := compiler.NewVariablesProvider(stored.CELTypeProvider())
	declOptions, err := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType).EnvOptions(variables)
	if err != nil {
		t.Fatalf("decl types: %v", err)
	}
	extended, err := base.Extend(
		environment.VersionedOptions{IntroducedVersion: vpolCompilerVersion, EnvOptions: append(opts, declOptions...)},
		environment.VersionedOptions{IntroducedVersion: vpolCompilerVersion, EnvOptions: []cel.EnvOption{image.Lib(image.Latest())}},
	)
	if err != nil {
		t.Fatalf("extend env: %v", err)
	}
	env, err := extended.Env(environment.StoredExpressions)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	return env, variables
}

// What Kyverno's vpol.Validate checks: every expression compiles with Kyverno's own compile
// functions, matchConstraints has a resource rule, and a namespaced policy reads no global context or HTTP.
func compileVpol(t *testing.T, pol *policiesv1.NamespacedValidatingPolicy) compiledPolicy {
	t.Helper()
	env, provider := vpolEnv(t)
	spec := pol.Spec
	path := field.NewPath("spec")
	conditions, errs := compiler.CompileMatchConditions(path.Child("matchConditions"), env, spec.MatchConditions...)
	assertNoFieldErrors(t, errs)
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, provider, spec.Variables...)
	assertNoFieldErrors(t, errs)
	validations := make([]compiler.Validation, constants.DefaultInitValue, len(spec.Validations))
	exprs := make([]string, constants.DefaultInitValue, len(spec.Validations))
	for i, v := range spec.Validations {
		program, errs := compiler.CompileValidation(path.Child("validations").Index(i), env, v)
		assertNoFieldErrors(t, errs)
		validations = append(validations, program)
		exprs = append(exprs, v.Expression, v.MessageExpression)
	}
	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == constants.DefaultInitValue {
		t.Error("Kyverno requires a matchConstraints resource rule")
	}
	for _, c := range spec.MatchConditions {
		exprs = append(exprs, c.Expression)
	}
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	if pol.Namespace == constants.EmptyString || compiler.ExpressionsUseGlobalContext(exprs...) || compiler.ExpressionsUseHTTP(exprs...) {
		t.Error("guard must validate the artifact we deploy: a namespaced policy without global context or HTTP")
	}
	return compiledPolicy{policy: pol, matchConditions: conditions, variables: variables, validations: validations}
}

func assertNoFieldErrors(t *testing.T, errs field.ErrorList) {
	t.Helper()
	if len(errs) > constants.DefaultInitValue {
		t.Fatalf("Kyverno would reject this policy: %v", errs.ToAggregate())
	}
}

// Rendering assertions only prove the shape we meant to emit; they cannot say whether Kyverno
// would accept it. Every rendered policy goes through Kyverno's compile functions here, in every
// template, scope and mode, with and without exclusions. No cluster needed.
func TestKyvernoAcceptsEveryRenderedPolicy(t *testing.T) {
	for _, mode := range []string{plans.ModeEnforce, plans.ModeAudit} {
		for _, tpl := range plans.Templates {
			for scopeType := range acceptScopes() {
				for _, excluded := range []bool{false, true} {
					t.Run(mode+"/"+tpl.ID+"/"+scopeType, func(t *testing.T) {
						plan, resolved := acceptPlan(tpl.ID, scopeType, mode, excluded)
						rendered, err := datapolicies.Render(plan, resolved, nil)
						if err != nil {
							t.Fatalf("render: %v", err)
						}
						if len(rendered) == constants.DefaultInitValue {
							t.Fatal("template rendered no policy")
						}
						for i := range rendered {
							compileVpol(t, &rendered[i])
							assertCRDShape(t, &rendered[i])
						}
					})
				}
			}
		}
	}
}

// The CRD lists validation actions as a set and wants no line break in a message.
func assertCRDShape(t *testing.T, pol *policiesv1.NamespacedValidatingPolicy) {
	t.Helper()
	if len(pol.Spec.ValidationAction) != constants.DefaultAddValue ||
		pol.Spec.ValidationAction[constants.DefaultInitValue] != admissionregistrationv1.Deny &&
			pol.Spec.ValidationAction[constants.DefaultInitValue] != admissionregistrationv1.Audit {
		t.Errorf("validation actions = %v, want exactly Deny or Audit", pol.Spec.ValidationAction)
	}
	for _, v := range pol.Spec.Validations {
		if v.Message == constants.EmptyString || strings.ContainsAny(v.Message, lineBreaks) {
			t.Errorf("message %q is empty or spans lines", v.Message)
		}
	}
}

// Exclusions add a kind entry, and for applications a named Deployment entry that also carries
// Deployment/scale. Kyverno must accept them all.
func acceptPlan(templateID, scopeType, mode string, excluded bool) (*plans.ProtectionPlan, map[string]datapolicies.ResolvedApp) {
	plan := &plans.ProtectionPlan{
		ID:       acceptPlanID,
		Name:     acceptPlanName,
		Mode:     mode,
		Scope:    plans.ProtectionPlanScope{Type: scopeType},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: templateID, Params: acceptParams[templateID]}},
	}
	if excluded {
		plan.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindDeployment, excludedKind}}
	}
	if scopeType == plans.ScopeTypeNamespaces {
		plan.Scope.Namespaces = []string{acceptNamespace}
		return plan, nil
	}
	plan.Scope.ApplicationRefs = []string{acceptApp}
	if excluded {
		plan.Scope.Exclusions.Resources = []plans.ProtectionPlanExcludedResource{
			{Kind: kindDeployment, Name: acceptApp, Namespace: acceptNamespace},
		}
	}
	scope := acceptScopes()[scopeType]
	return plan, map[string]datapolicies.ResolvedApp{
		acceptApp: {Namespaces: []string{acceptNamespace}, Resources: scope.AppResources, VolumeClaims: scope.VolumeClaims},
	}
}
