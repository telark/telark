package policies

import (
	"errors"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
)

func Render(plan *plans.ProtectionPlan, resolved map[string]ResolvedApp, logger Logger) ([]kyvernov1.Policy, error) {
	if plan == nil {
		return nil, errors.New("policies: plan is required")
	}

	scopes, err := buildScopes(plan, resolved)
	if err != nil {
		return nil, err
	}

	meta := RenderMeta{
		PlanID:    plan.ID,
		PlanName:  plan.Name,
		CreatedBy: plan.CreatedBy,
		Mode:      plan.Mode,
	}

	out := make([]kyvernov1.Policy, constants.DefaultInitValue, len(scopes)*len(plan.Policies))
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
			if pol == nil {
				logSkippedTemplate(logger, plan.ID, entry.TemplateID, scope)
				continue
			}
			out = append(out, *pol)
		}
	}
	return out, nil
}

func logSkippedTemplate(logger Logger, planID, templateID string, scope ScopeSpec) {
	if logger == nil {
		return
	}
	logger.Info(fmt.Sprintf(
		"template %s skipped for plan %s namespace=%s apps=%v: no matching application resources",
		templateID, planID, scope.Namespace, scope.ApplicationIDs,
	))
}

func buildScopes(plan *plans.ProtectionPlan, resolved map[string]ResolvedApp) ([]ScopeSpec, error) {
	switch plan.Scope.Type {
	case plans.ScopeTypeNamespaces:
		nss := append([]string(nil), plan.Scope.Namespaces...)
		slices.Sort(nss)
		out := make([]ScopeSpec, constants.DefaultInitValue, len(nss))
		for _, ns := range nss {
			out = append(out, ScopeSpec{Namespace: ns})
		}
		return out, nil

	case plans.ScopeTypeApplications:
		grouped := map[string][]string{}
		resourcesByNS := map[string][]ApplicationResourceRef{}
		claimsByNS := map[string][]string{}
		for _, appID := range plan.Scope.ApplicationIDs {
			ra, ok := resolved[appID]
			if !ok || ra.Namespace == constants.EmptyString {
				return nil, fmt.Errorf("policies: application %q has no resolved namespace", appID)
			}
			grouped[ra.Namespace] = append(grouped[ra.Namespace], appID)
			resourcesByNS[ra.Namespace] = append(resourcesByNS[ra.Namespace], ra.Resources...)
			claimsByNS[ra.Namespace] = append(claimsByNS[ra.Namespace], ra.VolumeClaims...)
		}
		nss := make([]string, constants.DefaultInitValue, len(grouped))
		for ns := range grouped {
			nss = append(nss, ns)
		}
		slices.Sort(nss)
		out := make([]ScopeSpec, constants.DefaultInitValue, len(nss))
		for _, ns := range nss {
			apps := grouped[ns]
			slices.Sort(apps)
			claims := claimsByNS[ns]
			slices.Sort(claims)
			out = append(out, ScopeSpec{
				Namespace:      ns,
				ApplicationIDs: apps,
				AppResources:   resourcesByNS[ns],
				VolumeClaims:   slices.Compact(claims),
			})
		}
		return out, nil

	default:
		return nil, fmt.Errorf("policies: unknown scope type %q", plan.Scope.Type)
	}
}
