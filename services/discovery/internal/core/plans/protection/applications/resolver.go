package applications

import (
	"context"
	"slices"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/policies"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
)

// Returns the resolved subset and the IDs that could not be located, so callers can surface a
// precise error. An application found only in ignored namespaces resolves with none.
type Resolver func(
	ctx context.Context,
	ids []string,
) (resolved map[string]policies.ResolvedApp, missing []string, err error)

func NewRedisResolver(rdb *redis.Client, readClaims ClaimReader) Resolver {
	return func(ctx context.Context, ids []string) (map[string]policies.ResolvedApp, []string, error) {
		apps, err := coordination.DiscoverApplications(ctx, rdb)
		if err != nil {
			return nil, nil, err
		}
		index := make(map[string]applicationmodel.Application, len(apps))
		for i := range apps {
			index[apps[i].Name] = apps[i]
		}

		ignored := validation.IgnoredNamespaces(ctx)
		resolved := make(map[string]policies.ResolvedApp, len(ids))
		var missing []string
		for _, id := range ids {
			app, ok := index[id]
			if !ok {
				missing = append(missing, id)
				continue
			}
			entry := policies.ResolvedApp{
				Namespaces: appNamespaces(app, ignored),
				Resources:  toResourceRefs(app.Resources),
			}
			if readClaims != nil {
				entry.VolumeClaims = readClaims(ctx, app.Resources)
			}
			resolved[id] = entry
		}
		return resolved, missing, nil
	}
}

func appNamespaces(app applicationmodel.Application, ignored []string) []string {
	out := make([]string, constants.DefaultInitValue, len(app.Namespaces.Items))
	for _, ns := range app.Namespaces.Items {
		if !slices.Contains(ignored, ns.Name) {
			out = append(out, ns.Name)
		}
	}
	return out
}

// Every namespace of the listed applications, in id order; ids absent from resolved are skipped.
func Namespaces(resolved map[string]policies.ResolvedApp, ids []string) []string {
	out := make([]string, constants.DefaultInitValue, len(ids))
	for _, id := range ids {
		out = append(out, resolved[id].Namespaces...)
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}

// Ids that resolved but have no namespace the policy engine evaluates.
func Unprotectable(resolved map[string]policies.ResolvedApp, ids []string) []string {
	return slices.DeleteFunc(slices.Clone(ids), func(id string) bool {
		ra, ok := resolved[id]
		return !ok || len(ra.Namespaces) > constants.DefaultInitValue
	})
}

func toResourceRefs(items []applicationmodel.Resource) []policies.ApplicationResourceRef {
	out := make([]policies.ApplicationResourceRef, constants.DefaultInitValue, len(items))
	for _, r := range items {
		out = append(out, policies.ApplicationResourceRef{
			Kind:      r.Kind,
			Name:      r.Name,
			Namespace: r.Namespace,
		})
	}
	return out
}
