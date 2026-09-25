package policies

import (
	"errors"
	"fmt"
	"maps"
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
		exclusions := ExclusionFilters(plan.Scope.Exclusions, scope.Namespace)
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
				if logger != nil {
					logger.Info(fmt.Sprintf(
						"template %s skipped for plan %s namespace=%s apps=%v: no matching application resources",
						entry.TemplateID, plan.ID, scope.Namespace, scope.ApplicationIDs,
					))
				}
				continue
			}
			applyExclusions(pol, exclusions)
			out = append(out, *pol)
		}
	}
	return out, nil
}

func buildScopes(plan *plans.ProtectionPlan, resolved map[string]ResolvedApp) ([]ScopeSpec, error) {
	switch plan.Scope.Type {
	case plans.ScopeTypeNamespaces:
		nss := slices.Sorted(slices.Values(plan.Scope.Namespaces))
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
		nss := slices.Sorted(maps.Keys(grouped))
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
