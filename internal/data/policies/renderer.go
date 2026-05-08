package policies

import (
	"fmt"
	"sort"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/plans"
)

// Callers must import "github.com/plsyro/data/policies/templates" once at
// startup (typically a blank import in main) so each template registers itself
// before Render runs. This split avoids a templates -> policies -> templates cycle.
func Render(plan *plans.ProtectionPlan, applicationNamespaces map[string]string) ([]kyvernov1.Policy, error) {
	if plan == nil {
		return nil, fmt.Errorf("policies: plan is required")
	}

	scopes, err := buildScopes(plan, applicationNamespaces)
	if err != nil {
		return nil, err
	}

	meta := RenderMeta{
		PlanID:    plan.ID,
		PlanName:  plan.Name,
		CreatedBy: plan.CreatedBy,
		Mode:      plan.Mode,
	}

	out := make([]kyvernov1.Policy, 0, len(scopes)*len(plan.Policies))
	for _, scope := range scopes {
		for _, entry := range plan.Policies {
			renderer, ok := GetRenderer(entry.TemplateID)
			if !ok {
				return nil, fmt.Errorf("policies: no renderer registered for template %q", entry.TemplateID)
			}
			pol, rerr := renderer.Render(meta, scope, entry.Params)
			if rerr != nil {
				return nil, fmt.Errorf("policies: render template %q: %w", entry.TemplateID, rerr)
			}
			out = append(out, *pol)
		}
	}
	return out, nil
}

func buildScopes(plan *plans.ProtectionPlan, applicationNamespaces map[string]string) ([]ScopeSpec, error) {
	switch plan.Scope.Type {
	case plans.ScopeTypeNamespaces:
		nss := append([]string(nil), plan.Scope.Namespaces...)
		sort.Strings(nss)
		out := make([]ScopeSpec, 0, len(nss))
		for _, ns := range nss {
			out = append(out, ScopeSpec{Namespace: ns})
		}
		return out, nil

	case plans.ScopeTypeApplications:
		grouped := map[string][]string{}
		for _, appID := range plan.Scope.ApplicationIds {
			ns, ok := applicationNamespaces[appID]
			if !ok || ns == "" {
				return nil, fmt.Errorf("policies: application %q has no resolved namespace", appID)
			}
			grouped[ns] = append(grouped[ns], appID)
		}
		nss := make([]string, 0, len(grouped))
		for ns := range grouped {
			nss = append(nss, ns)
		}
		sort.Strings(nss)
		out := make([]ScopeSpec, 0, len(nss))
		for _, ns := range nss {
			apps := grouped[ns]
			sort.Strings(apps)
			out = append(out, ScopeSpec{Namespace: ns, ApplicationIDs: apps})
		}
		return out, nil

	default:
		return nil, fmt.Errorf("policies: unknown scope type %q", plan.Scope.Type)
	}
}
