package policies

import (
	"slices"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
)

const (
	unregisteredID   = "no-such-template"
	unregisteredCode = "zz"
	longCodeID       = "long-code-template"
	// Long enough that telark-<plan id>-<code>-<scope hash> outgrows a Kubernetes name.
	longCode            = "a-template-code-far-too-long-for-a-name"
	tplBlockImageTags   = "block-image-tags"
	tplBlockImageTypes  = "block-image-types"
	tplBlockMounts      = "block-workload-config-mount-changes"
	paramTags           = "tags"
	paramImagePatterns  = "imagePatterns"
	tagLatest           = "latest"
	planScopeClusters   = "clusters"
	renderRefusalPlanID = "pp-refusal-1234-5678"
)

// Registered once per process: a second Register for the same ID panics.
type longCodeRenderer struct{}

func (longCodeRenderer) TemplateID() string   { return longCodeID }
func (longCodeRenderer) TemplateCode() string { return longCode }
func (longCodeRenderer) Render(policies.RenderMeta, policies.ScopeSpec, map[string]any) (*kyvernov1.Policy, error) {
	return nil, nil
}

type infoLog []string

func (l *infoLog) Info(msg string) { *l = append(*l, msg) }
func (*infoLog) Error(string)      {}

func withCatalog(t *testing.T, edit func([]plans.Template) []plans.Template) {
	t.Helper()
	saved := plans.Templates
	plans.Templates = edit(slices.Clone(saved))
	t.Cleanup(func() { plans.Templates = saved })
}

// Startup refuses a catalog that would render broken policies: a template with no renderer,
// a missing, shared or mismatched code, or a code too long for the Policy name it goes into.
func TestValidateCatalog(t *testing.T) {
	if msg := policies.ValidateCatalog(); msg != constants.EmptyString {
		t.Fatalf("the shipped catalog is refused: %s", msg)
	}
	if _, ok := policies.GetRenderer(longCodeID); !ok {
		policies.Register(longCodeRenderer{})
	}
	first, second := constants.DefaultInitValue, constants.SingleItem
	cases := []struct {
		name string
		edit func([]plans.Template) []plans.Template
		want string
	}{
		{"no renderer", func(c []plans.Template) []plans.Template {
			return append(c, plans.Template{ID: unregisteredID, Code: unregisteredCode})
		}, "no renderer registered"},
		{"empty code", func(c []plans.Template) []plans.Template { c[first].Code = constants.EmptyString; return c }, "has empty Code"},
		{"shared code", func(c []plans.Template) []plans.Template { c[second].Code = c[first].Code; return c }, "collides with"},
		{"mismatched code", func(c []plans.Template) []plans.Template { c[first].Code = unregisteredCode; return c }, "Code mismatch"},
		{"code too long", func(c []plans.Template) []plans.Template {
			return append(c, plans.Template{ID: longCodeID, Code: longCode})
		}, "exceeds K8s limit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withCatalog(t, c.edit)
			if msg := policies.ValidateCatalog(); !strings.Contains(msg, c.want) {
				t.Fatalf("ValidateCatalog = %q, want it to say %q", msg, c.want)
			}
		})
	}
}

// Two renderers for one template would make the rendered policy depend on init order.
func TestRegisterRefusesASecondRenderer(t *testing.T) {
	existing, ok := policies.GetRenderer(tplBlockUpdate)
	if !ok {
		t.Fatalf("no renderer for %s", tplBlockUpdate)
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("Register accepted a second renderer for %s", tplBlockUpdate)
		}
	}()
	policies.Register(existing)
}

func refusalPlan(scope plans.ProtectionPlanScope, templateID string, params map[string]any) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:       renderRefusalPlanID,
		Mode:     plans.ModeEnforce,
		Scope:    scope,
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: templateID, Params: params}},
	}
}

// A plan that can't be rendered whole renders nothing, so no half-applied plan reaches the cluster.
func TestRenderRefusesWhatItCannotRender(t *testing.T) {
	nsScope := plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{planNamespace}}
	appsScope := plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{appName}}
	tags := func(value any) map[string]any { return map[string]any{paramTags: value} }
	cases := []struct {
		name string
		plan *plans.ProtectionPlan
	}{
		{"no plan", nil},
		{"unknown scope type", refusalPlan(plans.ProtectionPlanScope{Type: planScopeClusters}, tplBlockUpdate, nil)},
		{"application without a namespace", refusalPlan(appsScope, tplBlockUpdate, nil)},
		{"unknown template", refusalPlan(nsScope, unregisteredID, nil)},
		{"param missing", refusalPlan(nsScope, tplBlockImageTags, nil)},
		{"param empty", refusalPlan(nsScope, tplBlockImageTags, tags([]string{}))},
		{"param listing nothing", refusalPlan(nsScope, tplBlockImageTags, tags([]any{}))},
		{"param listing a number", refusalPlan(nsScope, tplBlockImageTags, tags([]any{constants.SingleItem}))},
		{"param not a list", refusalPlan(nsScope, tplBlockImageTypes, map[string]any{paramImagePatterns: tagLatest})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if rendered, err := policies.Render(c.plan, nil, nil); err == nil {
				t.Fatalf("Render = %d policies, want a refusal", len(rendered))
			}
		})
	}
	// A list decoded from JSON arrives as []any and renders like []string.
	if _, err := policies.Render(refusalPlan(nsScope, tplBlockImageTags, tags([]any{tagLatest})), nil, nil); err != nil {
		t.Fatalf("Render with a decoded tag list: %v", err)
	}
}

// An application with nothing a template can match skips that template (and says so in the log)
// rather than rendering a policy that matches every resource; block-create still guards the app's
// identity, since what it blocks doesn't exist yet.
func TestRenderSkipsTemplatesWithoutTargets(t *testing.T) {
	skipped := []string{tplBlockUpdate, tplBlockStorageChanges, tplBlockMounts}
	entries := make([]plans.ProtectionPlanPolicy, constants.DefaultInitValue, len(skipped)+constants.SingleItem)
	for _, id := range append(slices.Clone(skipped), tplBlockCreate) {
		entries = append(entries, plans.ProtectionPlanPolicy{TemplateID: id})
	}
	plan := refusalPlan(plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{appName}}, tplBlockCreate, nil)
	plan.Policies = entries
	resolved := map[string]policies.ResolvedApp{appName: {Namespaces: []string{planNamespace}}}

	var log infoLog
	rendered, err := policies.Render(plan, resolved, &log)
	if err != nil {
		t.Fatalf(fmtRenderErr, err)
	}
	if len(rendered) != constants.SingleItem || len(rendered[constants.DefaultInitValue].Spec.Rules) != constants.SingleItem {
		t.Fatalf("rendered %d policies, want block-create alone with one rule", len(rendered))
	}
	match := rendered[constants.DefaultInitValue].Spec.Rules[constants.DefaultInitValue].MatchResources.Any
	if len(match) != constants.SingleItem || match[constants.DefaultInitValue].Selector == nil {
		t.Fatalf("block-create matches %+v, want the application's identity alone", match)
	}
	if len(log) != len(skipped) {
		t.Fatalf("logged %q, want one skip per template in %v", log, skipped)
	}
	for i, id := range skipped {
		if !strings.Contains(log[i], id) {
			t.Errorf("skip line %q does not name %s", log[i], id)
		}
	}
}
