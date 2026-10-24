package coreapps

import (
	"context"
	"slices"
	"testing"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/core"
	appshared "github.com/telark/telark/services/discovery/internal/core/applications/shared"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	apiVersionApps = "apps/v1"
	containerPort  = 8080
	keyAPIVersion  = "apiVersion"
	keyContainers  = "containers"
	namespaceProd  = "prod"
	republishes    = 5
	keyKind        = "kind"
	keyMetadata    = "metadata"
	keyName        = "name"
	keyNamespace   = "namespace"
	keySpec        = "spec"
	keyTemplate    = "template"
	labelApp       = "app"
)

// With every input served by the informer manifest cache the enriched
// fields come out populated without any kind fetcher running: there is no
// Kubernetes client here, so a fetcher would have returned nothing.
func TestDiscoverInputsSkipsFetchersOnCacheHit(t *testing.T) {
	hits := constants.DefaultInitValue
	core.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		hits++
		return map[string]any{
			keyAPIVersion: "apps/v1",
			keyKind:       kind,
			keyMetadata: map[string]any{
				keyName:             name,
				keyNamespace:        ns,
				"creationTimestamp": "2026-09-01T10:00:00Z",
				"annotations":       map[string]any{"telark.io/last-modified-by": "alice"},
			},
			keySpec: map[string]any{keyTemplate: map[string]any{keySpec: map[string]any{
				"containers": []any{map[string]any{
					keyName: "web",
					"image": "nginx:1.27",
					"ports": []any{map[string]any{"containerPort": int64(containerPort)}},
				}},
			}}},
		}, true
	}
	t.Cleanup(func() { core.GetManifestFromCache = nil })

	out := core.DiscoverInputsWithK8s(context.Background(), []derivation.ResourceInput{
		{Namespace: "prod", Kind: appshared.KindDeployment, Name: "web"},
	})

	testutil.Equal(t, "cache hits", hits, constants.DefaultAddValue)
	testutil.Equal(t, "createdAt set", out[0].CreatedAt.IsZero(), false)
	testutil.Equal(t, "lastModifiedBy", out[0].LastModifiedBy, "alice")
	testutil.Equal(t, "images", len(out[0].Images), constants.DefaultAddValue)
	testutil.Equal(t, "ports", len(out[0].Ports), constants.DefaultAddValue)
}

func podSpec() map[string]any {
	return map[string]any{"containers": []any{map[string]any{
		keyName: "job",
		"image": "busybox:1.36",
		"env":   []any{map[string]any{keyName: "FOO", "value": "bar"}},
	}}}
}

// A CronJob's pod spec sits under jobTemplate and a Job's under template; neither
// was read, so their apps showed no images, env keys or config refs.
func TestDiscoverInputsReadsJobAndCronJobPodSpecs(t *testing.T) {
	core.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		spec := map[string]any{keyTemplate: map[string]any{keySpec: podSpec()}}
		if kind == appshared.KindCronJob {
			spec = map[string]any{"jobTemplate": map[string]any{keySpec: spec}}
		}
		return map[string]any{
			keyAPIVersion: "batch/v1",
			keyKind:       kind,
			keyMetadata:   map[string]any{keyName: name, keyNamespace: ns},
			keySpec:       spec,
		}, true
	}
	t.Cleanup(func() { core.GetManifestFromCache = nil })

	out := core.DiscoverInputsWithK8s(context.Background(), []derivation.ResourceInput{
		{Namespace: "prod", Kind: appshared.KindCronJob, Name: "nightly"},
		{Namespace: "prod", Kind: appshared.KindJob, Name: "once"},
	})
	for i := range out {
		testutil.Equal(t, out[i].Kind+" images", len(out[i].Images), constants.DefaultAddValue)
		testutil.Equal(t, out[i].Kind+" env keys", len(out[i].EnvVarKeys), constants.DefaultAddValue)
	}
}

// A grouping label the CR name rules refuse (exporter 422) looped a publish and
// an orphan baseline snapshot every tick; the app is skipped instead.
func TestGetApplicationsSkipsNamesThatCannotNameACR(t *testing.T) {
	core.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		return map[string]any{
			keyAPIVersion: "apps/v1",
			keyKind:       kind,
			keyMetadata:   map[string]any{keyName: name, keyNamespace: ns},
			keySpec:       map[string]any{keyTemplate: map[string]any{keySpec: podSpec()}},
		}, true
	}
	t.Cleanup(func() { core.GetManifestFromCache = nil })
	inputs := []derivation.ResourceInput{
		{Namespace: "prod", Kind: appshared.KindDeployment, Name: "bad", Labels: map[string]string{"app": "bad name"}},
		{Namespace: "prod", Kind: appshared.KindDeployment, Name: "good", Labels: map[string]string{"app": "Good_Name"}},
	}

	res := core.GetApplications(context.Background(), nil, inputs, core.GetApplicationsOptions{DeriveOnly: true})
	data, ok := res.Data.(application.ResponseData)
	testutil.Equal(t, "response shape", ok, true)
	testutil.Equal(t, "one application", len(data.Applications), constants.DefaultAddValue)
	testutil.Equal(t, "normalized name", data.Applications[constants.DefaultInitValue].Name, "good-name")
}

// The lists came from ranging over sets, so every publish reshuffled them on the app page.
func TestGetApplicationsSortsImagesPortsAndEnvKeys(t *testing.T) {
	images := []string{"zipkin:3", "busybox:1.36", "nginx:1.27", "redis:8"}
	envKeys := []string{"ZED", "ALPHA", "MID", "BETA"}
	core.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		containers := make([]any, constants.DefaultInitValue, len(images))
		for i, image := range images {
			containers = append(containers, map[string]any{
				keyName: envKeys[i],
				"image": image,
				"env":   []any{map[string]any{keyName: envKeys[i], "value": "x"}},
				// Descending, so an unsorted result can't pass by insertion order.
				"ports": []any{map[string]any{"containerPort": int64(containerPort - i)}},
			})
		}
		return map[string]any{
			keyAPIVersion: apiVersionApps,
			keyKind:       kind,
			keyMetadata:   map[string]any{keyName: name, keyNamespace: ns},
			keySpec:       map[string]any{keyTemplate: map[string]any{keySpec: map[string]any{keyContainers: containers}}},
		}, true
	}
	t.Cleanup(func() { core.GetManifestFromCache = nil })
	inputs := []derivation.ResourceInput{
		{Namespace: namespaceProd, Kind: appshared.KindDeployment, Name: "shop", Labels: map[string]string{labelApp: "shop"}},
	}

	for range republishes {
		res := core.GetApplications(context.Background(), nil, inputs, core.GetApplicationsOptions{DeriveOnly: true})
		data, ok := res.Data.(application.ResponseData)
		testutil.Equal(t, "response shape", ok, true)
		app := data.Applications[constants.DefaultInitValue]
		testutil.Equal(t, "images", len(app.Images), len(images))
		testutil.Equal(t, "images sorted", slices.IsSorted(app.Images), true)
		testutil.Equal(t, "ports sorted", slices.IsSorted(app.Ports), true)
		testutil.Equal(t, "env keys sorted", slices.IsSorted(app.EnvVarKeys), true)
	}
}
