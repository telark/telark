package policies

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
)

const (
	injectedPlanName = "{{request.object}}"
	lineBreaks       = "\r\n"
	regexPrefix      = "^(?:"
	globFill         = "zz"
	// The claim template and the image pattern.
	wantGlobRegexes = 2
)

// Strings a user or the cluster controls, each built to break out of a CEL string literal.
var hostileValues = []string{`x" || true || "`, `a\" + string(object) + \"b`, `{{request.object.data}}`, "tab\there", `ü-ñ`}

// A CEL policy prints its message as written, so the plan is named as the user wrote it: quoted,
// on one line, untouched by the audit wording, and the message expression starts with that same text.
func TestMessagesNameThePlan(t *testing.T) {
	env := celEnv(t)
	for _, name := range []string{injectedPlanName, `x is blocked by "y"`, "two\nlines", constants.EmptyString} {
		for _, mode := range []string{plans.ModeEnforce, plans.ModeAudit} {
			plan := &plans.ProtectionPlan{
				ID:       multiPlanID,
				Name:     name,
				Mode:     mode,
				Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{planNamespace}},
				Policies: allTemplates(),
			}
			rendered, err := policies.Render(plan, nil, nil)
			if err != nil {
				t.Fatalf(fmtRenderErr, err)
			}
			want := strconv.Quote(name)
			if name == constants.EmptyString {
				want = strconv.Quote(multiPlanID)
			}
			for i := range rendered {
				for _, v := range rendered[i].Spec.Validations {
					assertNamedMessage(t, env, v.Message, v.MessageExpression, want)
				}
			}
		}
	}
}

func assertNamedMessage(t *testing.T, env *cel.Env, message, messageExpression, want string) {
	t.Helper()
	if !strings.HasSuffix(message, " plan "+want+".") || strings.ContainsAny(message, lineBreaks) {
		t.Errorf("message %q does not end naming the plan %s on one line", message, want)
	}
	if lits := literals(t, env, messageExpression); len(lits) == constants.DefaultInitValue || lits[constants.DefaultInitValue] != message {
		t.Errorf("message expression %q does not start with the message", messageExpression)
	}
}

func allTemplates() []plans.ProtectionPlanPolicy {
	entries := make([]plans.ProtectionPlanPolicy, constants.DefaultInitValue, len(plans.Templates))
	for _, tpl := range plans.Templates {
		entries = append(entries, plans.ProtectionPlanPolicy{TemplateID: tpl.ID, Params: templateParams[tpl.ID]})
	}
	return entries
}

func celEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := cel.NewEnv(cel.OptionalTypes())
	if err != nil {
		t.Fatalf("cel env: %v", err)
	}
	return env
}

func hostilePlan(value string) (*plans.ProtectionPlan, map[string]policies.ResolvedApp) {
	entries := make([]plans.ProtectionPlanPolicy, len(plans.Templates))
	for i, tpl := range plans.Templates {
		entries[i] = plans.ProtectionPlanPolicy{TemplateID: tpl.ID}
	}
	params := map[string]map[string]any{
		"block-image-types": {"imagePatterns": []string{value + "*"}},
		"block-image-tags":  {"tags": []string{value}},
	}
	for i := range entries {
		entries[i].Params = params[entries[i].TemplateID]
	}
	plan := &plans.ProtectionPlan{
		ID:   multiPlanID,
		Name: value,
		Mode: plans.ModeEnforce,
		Scope: plans.ProtectionPlanScope{
			Type:            plans.ScopeTypeApplications,
			ApplicationRefs: []string{value},
			Exclusions: &plans.ProtectionPlanScopeExclusions{
				Kinds:     []string{value},
				Resources: []plans.ProtectionPlanExcludedResource{{Kind: kindConfigMap, Name: value, Namespace: planNamespace}},
			},
		},
		Policies: entries,
	}
	kinds := []string{kindDeployment, "StatefulSet", kindConfigMap, policies.KindCronJob}
	resources := make([]policies.ApplicationResourceRef, constants.DefaultInitValue, len(kinds))
	for _, kind := range kinds {
		resources = append(resources, policies.ApplicationResourceRef{Kind: kind, Name: value, Namespace: planNamespace})
	}
	return plan, map[string]policies.ResolvedApp{
		value: {Namespaces: []string{planNamespace}, Resources: resources, VolumeClaims: []string{value + "-*"}},
	}
}

func expressions(pol *policiesv1.NamespacedValidatingPolicy) []string {
	out := make([]string, constants.DefaultInitValue, len(pol.Spec.MatchConditions)+len(pol.Spec.Variables)+len(pol.Spec.Validations))
	for _, c := range pol.Spec.MatchConditions {
		out = append(out, c.Expression)
	}
	for _, v := range pol.Spec.Variables {
		out = append(out, v.Expression)
	}
	for _, v := range pol.Spec.Validations {
		out = append(out, v.Expression, v.MessageExpression)
	}
	return slices.DeleteFunc(out, func(e string) bool { return e == "" })
}

func literals(t *testing.T, env *cel.Env, expr string) []string {
	t.Helper()
	ast, iss := env.Parse(expr)
	if iss.Err() != nil {
		t.Fatalf("CEL %q does not parse: %v", expr, iss.Err())
	}
	var out []string
	celast.PreOrderVisit(ast.NativeRep().Expr(), celast.NewExprVisitor(func(e celast.Expr) {
		if e.Kind() == celast.LiteralKind {
			if s, ok := e.AsLiteral().Value().(string); ok {
				out = append(out, s)
			}
		}
	}))
	return out
}

// Every name, kind, label value, tag and pattern reaches CEL as one literal that decodes back to
// exactly what the user wrote, so none can end the literal and add an expression of its own.
func TestUserStringsReachCELLiterally(t *testing.T) {
	env := celEnv(t)
	for _, value := range hostileValues {
		plan, resolved := hostilePlan(value)
		rendered, err := policies.Render(plan, resolved, nil)
		if err != nil {
			t.Fatalf(fmtRenderErr, err)
		}
		var all []string
		for i := range rendered {
			for _, expr := range expressions(&rendered[i]) {
				all = append(all, literals(t, env, expr)...)
			}
		}
		if !slices.Contains(all, value) {
			t.Errorf("%q never came back as a literal", value)
		}
		if globs := literalGlobs(t, all, value); globs < wantGlobRegexes {
			t.Errorf("%q: %d glob regexes kept it literal, want %d", value, globs, wantGlobRegexes)
		}
	}
}

// The claim template and the image pattern come back as anchored regexes whose fixed part is literal.
func literalGlobs(t *testing.T, all []string, value string) int {
	t.Helper()
	var globs int
	for _, lit := range all {
		re, err := regexp.Compile(lit)
		if !strings.HasPrefix(lit, regexPrefix) || err != nil {
			continue
		}
		if re.MatchString(value+globFill) || re.MatchString(value+"-"+globFill) {
			globs++
		}
		if re.MatchString("x" + value + globFill) {
			t.Errorf("regex %q matches text before %q", lit, value)
		}
	}
	return globs
}
