package cache

import (
	"context"
	"encoding/json"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/discovery/shared"
	"github.com/redis/go-redis/v9"
)

func EnqueueJob(ctx context.Context, rdb *redis.Client, app application.Application) error {
	if rdb == nil {
		return nil
	}
	images := shared.DefaultSlice(app.Images, []string{})
	ports := shared.DefaultSlice(app.Ports, []int{})
	envVarKeys := shared.DefaultSlice(app.EnvVarKeys, []string{})
	resourceKinds := dedupeKinds(app.Resources)
	hasIngress := app.ResourceSummary.Ingress > hasAnyCount
	hasPVC := app.ResourceSummary.PersistentVolumeClaim > hasAnyCount

	primaryNamespace := constants.EmptyString
	if len(app.Namespaces.Items) > firstItemIdx {
		primaryNamespace = app.Namespaces.Items[firstItemIdx].Name
	}

	signals := AppSignals{
		Name:          app.Name,
		Namespace:     primaryNamespace,
		Images:        images,
		Ports:         ports,
		EnvVarKeys:    envVarKeys,
		ResourceKinds: resourceKinds,
		HasIngress:    hasIngress,
		HasPVC:        hasPVC,
	}
	payload, err := json.Marshal(signals)
	if err != nil {
		return err
	}
	return rdb.LPush(ctx, queueKey, payload).Err()
}

func dedupeKinds(resources []application.Resource) []string {
	seen := make(map[string]bool)
	out := make([]string, constants.DefaultInitValue)
	for _, r := range resources {
		if !seen[r.Kind] {
			seen[r.Kind] = true
			out = append(out, r.Kind)
		}
	}
	return out
}
