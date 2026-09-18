package applications

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/policies"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
)

// Returns the resolved subset and the IDs that could not be located, so callers can surface a
// precise error.
type Resolver func(
	ctx context.Context,
	ids []string,
) (resolved map[string]policies.ResolvedApp, missing []string, err error)

func NewRedisResolver(rdb *redis.Client) Resolver {
	return func(ctx context.Context, ids []string) (map[string]policies.ResolvedApp, []string, error) {
		apps, err := coordination.DiscoverApplications(ctx, rdb)
		if err != nil {
			return nil, nil, err
		}
		index := make(map[string]applicationmodel.Application, len(apps))
		for i := range apps {
			index[apps[i].Name] = apps[i]
		}

		resolved := make(map[string]policies.ResolvedApp, len(ids))
		var missing []string
		for _, id := range ids {
			app, ok := index[id]
			if !ok {
				missing = append(missing, id)
				continue
			}
			ns := firstNamespace(app)
			if ns == constants.EmptyString {
				missing = append(missing, id)
				continue
			}
			resolved[id] = policies.ResolvedApp{
				Namespace: ns,
				Resources: toResourceRefs(app.Resources),
			}
		}
		return resolved, missing, nil
	}
}

func firstNamespace(app applicationmodel.Application) string {
	if len(app.Namespaces.Items) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	return app.Namespaces.Items[constants.DefaultInitValue].Name
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
