package policies

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/kyverno/sdk/extensions/cel/compiler"
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
	"k8s.io/apiserver/pkg/cel/library"
)

const (
	acceptNamespace = "prod"
	acceptApp       = "wa1"
	acceptPlanID    = "pp-abc-1234-5678"
	acceptPlanName  = "freeze"
	kindDeployment  = "Deployment"
	lineBreaks      = "\r\n"
	orValueMacro    = "orValue"
	orValueCompat   = "__orValueCompat__"

	// The CEL library versions Kyverno 1.19.2's env pins.
	kyvernoListsVersion   = 3
	kyvernoStringsVersion = 2
	kyvernoRegexVersion   = 1
	kyvernoSemverVersion  = 1
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

// The environment of Kyverno 1.19's vpol compiler (createBaseVpolEnv), on the light sdk module: Kyverno's
// own module links cosign and three cloud SDKs. Of Kyverno's libraries the policies call only the image one.
func vpolEnv(t *testing.T) (*cel.Env, *compiler.VariablesProvider) {
	t.Helper()
	opts := append(
		kyvernoEnvOptions(),
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

// Kyverno 1.19.2's DefaultEnvOptionsWithCompat (pkg/cel/compiler/env.go), which the sdk lacks.
// Compare it with Kyverno's on every Kyverno bump.
func kyvernoEnvOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.HomogeneousAggregateLiterals(),
		cel.EagerlyValidateDeclarations(true),
		cel.DefaultUTCTimeZone(true),
		cel.CrossTypeNumericComparisons(true),
		cel.OptionalTypes(),
		ext.Bindings(),
		ext.Encoders(),
		ext.Lists(ext.ListsVersion(kyvernoListsVersion)),
		ext.Math(),
		ext.Protos(),
		ext.Sets(),
		ext.Strings(ext.StringsVersion(kyvernoStringsVersion)),
		ext.Regex(ext.RegexVersion(kyvernoRegexVersion)),
		library.CIDR(),
		library.Format(),
		library.IP(),
		library.Lists(),
		library.Regex(),
		library.URLs(),
		library.Quantity(),
		library.SemverLib(library.SemverVersion(kyvernoSemverVersion)),
		// cel-go 0.28 made orValue on a concrete value an error, and Kyverno keeps the old behavior.
		cel.Macros(cel.ReceiverMacro(orValueMacro, constants.DefaultAddValue, orValueCompatMacro)),
		cel.Function(orValueCompat, cel.Overload(orValueCompat+"_dyn_dyn",
			[]*cel.Type{cel.DynType, cel.DynType}, cel.DynType, cel.BinaryBinding(orValueOf))),
	}
}

func orValueCompatMacro(eh cel.MacroExprFactory, target celast.Expr, args []celast.Expr) (celast.Expr, *cel.Error) {
	return eh.NewCall(orValueCompat, target, args[constants.DefaultInitValue]), nil
}

// Kyverno passes its resource-limit errors through here, which this env, without its cost library, never raises.
func orValueOf(lhs, rhs ref.Val) ref.Val {
	switch v := lhs.(type) {
	case *types.Optional:
		if v.HasValue() {
			return v.GetValue()
		}
		return rhs
	case *types.Err:
		return rhs
	default:
		return lhs
	}
}

// What Kyverno's vpol.Validate checks: every expression compiles, matchConstraints has a resource rule,
// and the policy is namespaced. The env declares no globalContext or http, so a use fails to compile.
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
	for i, v := range spec.Validations {
		program, errs := compiler.CompileValidation(path.Child("validations").Index(i), env, v)
		assertNoFieldErrors(t, errs)
		validations = append(validations, program)
	}
	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == constants.DefaultInitValue {
		t.Error("Kyverno requires a matchConstraints resource rule")
	}
	if pol.Namespace == constants.EmptyString {
		t.Error("guard must validate the artifact we deploy: a namespaced policy")
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
			for _, scopeType := range supportedScopes(tpl) {
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

// Only the scopes the API lets a plan pick: prepare refuses the rest.
func supportedScopes(tpl plans.Template) []string {
	var out []string
	for scopeType := range acceptScopes() {
		if slices.Contains(tpl.SupportedScopes, plans.ScopeSupport(scopeType)) {
			out = append(out, scopeType)
		}
	}
	return out
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
