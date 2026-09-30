package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/kcore/resources/workload"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/diff"
	"github.com/telark/telark/services/discovery/internal/core/applications/metrics"
	appshared "github.com/telark/telark/services/discovery/internal/core/applications/shared"
	"github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	"k8s.io/apimachinery/pkg/util/validation"
)

// applyHistoryFromDiff returns, per application, what the diff did with the
// history and snapshots it produced. Callers without an informer-captured
// pre-image echo back stored values, which must not be republished.
func applyHistoryFromDiff(
	ctx context.Context,
	apps *[]application.Application,
	opts GetApplicationsOptions,
) ([]diff.Outcome, []*application.Application) {
	outcomes := make([]diff.Outcome, len(*apps))
	storedApps := make([]*application.Application, len(*apps))
	for i := range *apps {
		app := &(*apps)[i]
		h, snaps, outcome, stored := historyAndSnapshotsForApp(ctx, app, opts)
		app.History = h
		app.Snapshots = snaps
		outcomes[i] = outcome
		storedApps[i] = stored
		// Nothing authored means the publish omits snapshots, leaving the prewritten files unreferenced.
		if outcome != diff.OutcomeAuthored && isPrewrittenForApp(app.Name, opts) {
			snapshot.DiscardSnapshots(opts.PrewrittenSnapshots, opts.DeleteSnapshot)
		}
		snapshot.NormalizeApplicationSnapshotTakenAt(app)
	}
	return outcomes, storedApps
}

// unchangedSinceStored is true when a publish would carry nothing the store
// does not already hold. Every no-op update still costs the exporter a CR write;
// at hundreds of apps per minute that queue is what starves real changes.
func unchangedSinceStored(stored *application.Application, fresh *application.Application) bool {
	return stored != nil &&
		stored.Health == fresh.Health &&
		stored.ResourceCount == fresh.ResourceCount &&
		reflect.DeepEqual(stored.Resources, fresh.Resources) &&
		reflect.DeepEqual(stored.Metrics, fresh.Metrics)
}

func historyAndSnapshotsForApp(
	ctx context.Context,
	app *application.Application,
	opts GetApplicationsOptions,
) (h application.ApplicationHistory, snaps []application.ApplicationSnapshot, outcome diff.Outcome, stored *application.Application) {
	defer func() {
		if r := recover(); r != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(string(constants.ErrApplicationHistoryDiffPanic), app.Name, r),
			)
			h = diff.NewApplicationHistory()
			snaps = []application.ApplicationSnapshot{}
			outcome = diff.OutcomeNoChange
		}
	}()
	if opts.GetStoredApplication != nil {
		var err error
		stored, err = opts.GetStoredApplication(app.Name)
		if err != nil {
			return diff.NewApplicationHistory(), []application.ApplicationSnapshot{}, diff.OutcomeNoChange, nil
		}
	}
	dopts := buildDiffOpts(app, opts)
	h, snaps, outcome = diff.DiffApplications(
		ctx,
		workload.ReadMetricsBaseline,
		opts.CreateSnapshot,
		opts.GetSnapshotManifest,
		opts.RedisClient,
		stored,
		*app,
		dopts,
	)
	if stored == nil && opts.GetSnapshotManifest != nil && len(snaps) > constants.DefaultInitValue {
		diff.HydrateApplicationFromV1Snapshots(ctx, app, snaps, opts.GetSnapshotManifest)
	}
	return h, snaps, outcome, stored
}

func buildDiffOpts(app *application.Application, opts GetApplicationsOptions) *diff.DiffOptions {
	hasPrewritten := isPrewrittenForApp(app.Name, opts)
	if !opts.FromCoalescingFlush && !opts.FromForceSync && !hasPrewritten {
		return nil
	}
	dopts := &diff.DiffOptions{
		ManifestPairs:       opts.ManifestPairs,
		FromCoalescingFlush: opts.FromCoalescingFlush,
		FromForceSync:       opts.FromForceSync,
		DeleteSnapshot:      opts.DeleteSnapshot,
		Rollback:            opts.Rollback,
	}
	if hasPrewritten {
		dopts.PrewrittenGeneration = opts.PrewrittenSnapshotGeneration
		dopts.PrewrittenSnapshots = opts.PrewrittenSnapshots
	}
	return dopts
}

func isPrewrittenForApp(appName string, opts GetApplicationsOptions) bool {
	return opts.PrewrittenSnapshotAppName != constants.EmptyString &&
		appName == opts.PrewrittenSnapshotAppName &&
		opts.PrewrittenSnapshotGeneration > constants.DefaultInitValue &&
		len(opts.PrewrittenSnapshots) > constants.DefaultInitValue
}

func filterHelmInternalSecrets(inputs []derivation.ResourceInput) []derivation.ResourceInput {
	out := make([]derivation.ResourceInput, constants.DefaultInitValue, len(inputs))
	for _, r := range inputs {
		if r.Kind == appshared.KindSecret && strings.HasPrefix(r.Name, helmReleaseSecretPrefix) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func GetApplications(
	ctx context.Context,
	rdb *redis.Client,
	inputs []derivation.ResourceInput,
	opts GetApplicationsOptions,
) response.GenericResponse {
	if opts.RedisClient == nil {
		opts.RedisClient = rdb
	}
	inputs = filterHelmInternalSecrets(inputs)
	inputs = DiscoverInputsWithK8s(ctx, inputs)
	withGroups := derivation.GroupByWorkloadAnchor(inputs)
	apps := buildApplications(withGroups, opts)
	totalResources := constants.DefaultInitValue
	for i := range apps {
		apps[i].Health = ComputeHealth(&apps[i])
		totalResources += apps[i].ResourceCount
	}

	if !opts.DeriveOnly {
		outcomes, storedApps := applyHistoryFromDiff(ctx, &apps, opts)
		for i := range apps {
			metrics.PopulateApplicationMetrics(&apps[i])
			if outcomes[i] == diff.OutcomeNoChange && unchangedSinceStored(storedApps[i], &apps[i]) {
				outcomes[i] = diff.OutcomeDeferred
			}
		}
		PublishApplications(opts.NatsClient, apps, outcomes)
	}

	return response.GenericResponse{
		Status:    http.StatusOK,
		Operation: operationSuccess,
		Message:   messageResourcesGroupedByApp,
		Data: application.ResponseData{
			TotalResources:    totalResources,
			TotalApplications: len(apps),
			Applications:      apps,
		},
	}
}

func buildApplications(
	withGroups []derivation.ResourceWithGroup,
	opts GetApplicationsOptions,
) []application.Application {
	if len(withGroups) == constants.DefaultInitValue {
		return []application.Application{}
	}
	byApp := make(map[string][]derivation.ResourceWithGroup)
	for _, r := range withGroups {
		byApp[r.Group] = append(byApp[r.Group], r)
	}
	order := discoveryshared.OrderedGroupNames(withGroups)
	out := make([]application.Application, constants.DefaultInitValue, len(order))
	for _, name := range order {
		if !validAppName(name) {
			continue
		}
		resources := byApp[name]
		var stored *application.Application
		if opts.GetStoredApplication != nil {
			var err error
			stored, err = opts.GetStoredApplication(name)
			if err != nil {
				if errors.Is(err, ErrNotJobTarget) {
					continue
				}
				constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
					fmt.Sprintf(string(constants.WarnHistoryStoredLookupFailed), name, err))
				continue
			}
		}
		out = append(out, buildApplication(name, resources, stored))
	}
	return out
}

var invalidAppNamesWarned sync.Map

// The exporter refuses a CR whose name is not a DNS-1123 subdomain; publishing one anyway
// looped a 422 and an orphan baseline snapshot every tick.
func validAppName(name string) bool {
	if len(validation.IsDNS1123Subdomain(name)) == constants.DefaultInitValue {
		return true
	}
	if _, warned := invalidAppNamesWarned.LoadOrStore(name, struct{}{}); !warned {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnHistoryAppNameInvalid), name))
	}
	return false
}

func aggregateFromResources(resources []derivation.ResourceWithGroup) resourceAggregate {
	agg := resourceAggregate{
		nsCounts:        make(map[string]int),
		kindCounts:      make(map[string]int),
		resList:         make([]application.Resource, constants.DefaultInitValue, len(resources)),
		imagesSet:       make(map[string]bool),
		portsSet:        make(map[int]bool),
		envKeysSet:      make(map[string]bool),
		configMapRefs:   make(map[string]bool),
		secretRefs:      make(map[string]bool),
		serviceMappings: make(map[string]bool),
		ingressRules:    make(map[string]bool),
	}
	for _, r := range resources {
		aggregateOneResource(&agg, r)
	}
	return agg
}

func aggregateOneResource(agg *resourceAggregate, r derivation.ResourceWithGroup) {
	agg.nsCounts[r.Namespace]++
	agg.kindCounts[r.Kind]++
	agg.resList = append(agg.resList, application.Resource{
		Namespace: r.Namespace,
		Kind:      r.Kind,
		Name:      r.Name,
	})
	if !r.CreatedAt.IsZero() {
		if agg.createdAt.IsZero() || r.CreatedAt.Before(agg.createdAt) {
			agg.createdAt = r.CreatedAt
		}
		if r.CreatedAt.After(agg.lastUpdated) {
			agg.lastUpdated = r.CreatedAt
		}
	}
	aggregateLastModified(agg, r)
	for _, img := range r.Images {
		agg.imagesSet[img] = true
	}
	for _, p := range r.Ports {
		agg.portsSet[p] = true
	}
	for _, k := range r.EnvVarKeys {
		agg.envKeysSet[k] = true
	}
	for _, ref := range r.ConfigMapRefs {
		agg.configMapRefs[ref] = true
	}
	for _, ref := range r.SecretRefs {
		agg.secretRefs[ref] = true
	}
	for _, mapping := range r.ServiceMappings {
		agg.serviceMappings[mapping] = true
	}
	for _, rule := range r.IngressRules {
		agg.ingressRules[rule] = true
	}
}

func aggregateLastModified(agg *resourceAggregate, r derivation.ResourceWithGroup) {
	if r.LastModifiedAt.IsZero() {
		return
	}
	key := r.Namespace + constants.PathSeparator + r.Kind + constants.PathSeparator + r.Name
	if agg.lastModifiedAt.IsZero() ||
		r.LastModifiedAt.After(agg.lastModifiedAt) ||
		(r.LastModifiedAt.Equal(agg.lastModifiedAt) &&
			(agg.lastModifiedKey == constants.EmptyString || key < agg.lastModifiedKey)) {
		agg.lastModifiedAt = r.LastModifiedAt
		agg.lastModifiedBy = r.LastModifiedBy
		agg.lastModifiedKey = key
	}
}

func buildApplication(
	name string,
	resources []derivation.ResourceWithGroup,
	stored *application.Application,
) application.Application {
	agg := aggregateFromResources(resources)
	appCreatedAt := agg.createdAt
	appLastUpdated := agg.lastUpdated
	if appCreatedAt.IsZero() {
		appCreatedAt = time.Now()
	}
	if appLastUpdated.IsZero() {
		appLastUpdated = time.Now()
	}
	h := diff.NewApplicationHistory()
	h.LastModifiedBy = agg.lastModifiedBy
	h.LastModifiedAt = formatAppTime(agg.lastModifiedAt)
	lv := labelsFromResources(resources)
	displayName := discoveryshared.BuildDisplayName(name)
	if stored != nil && stored.DisplayName != constants.EmptyString {
		displayName = stored.DisplayName
	}
	nsItems := discoveryshared.BuildNamespaceItems(agg.nsCounts)
	managed := discoveryshared.BuildManaged(lv.managedBy, lv.chartVal, lv.versionVal)
	summary := discoveryshared.ResourceSummaryFromKindCounts(agg.kindCounts)
	images := setToSlice(agg.imagesSet)
	ports := intSetToSlice(agg.portsSet)
	envVarKeys := setToSlice(agg.envKeysSet)
	configMapRefs := setToSlice(agg.configMapRefs)
	secretRefs := setToSlice(agg.secretRefs)
	serviceMappings := setToSlice(agg.serviceMappings)
	ingressRules := setToSlice(agg.ingressRules)
	return application.Application{
		Name:            name,
		DisplayName:     displayName,
		Health:          application.Health{}, // set by ComputeHealth in GetApplications
		ResourceCount:   len(resources),
		Namespaces:      application.Namespaces{Total: len(agg.nsCounts), Items: nsItems},
		Managed:         managed,
		CreatedAt:       formatAppTime(appCreatedAt),
		LastUpdated:     formatAppTime(appLastUpdated),
		ResourceSummary: summary,
		Resources:       agg.resList,
		Images:          images,
		Ports:           ports,
		EnvVarKeys:      envVarKeys,
		ConfigMapRefs:   configMapRefs,
		SecretRefs:      secretRefs,
		ServiceMappings: serviceMappings,
		IngressRules:    ingressRules,
		Snapshots:       []application.ApplicationSnapshot{},
		Metrics:         application.NewApplicationMetrics(),
		History:         h,
	}
}

func labelsFromResources(resources []derivation.ResourceWithGroup) labelValues {
	var out labelValues
	for _, r := range resources {
		if r.Labels == nil {
			continue
		}
		if out.managedBy == constants.EmptyString && r.Labels[labelManagedBy] != constants.EmptyString {
			out.managedBy = r.Labels[labelManagedBy]
		}
		if out.chartVal == constants.EmptyString && r.Labels[labelChart] != constants.EmptyString {
			out.chartVal = r.Labels[labelChart]
		}
		if out.versionVal == constants.EmptyString && r.Labels[labelVersion] != constants.EmptyString {
			out.versionVal = r.Labels[labelVersion]
		}
		if out.displayName == constants.EmptyString && r.Labels[labelAppName] != constants.EmptyString {
			out.displayName = r.Labels[labelAppName]
		}
	}
	return out
}

// ErrNotJobTarget marks an application a per-app job must skip silently.
var ErrNotJobTarget = errors.New(string(constants.ErrPrewarmNotJobTarget))
