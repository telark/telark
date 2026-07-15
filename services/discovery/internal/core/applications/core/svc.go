package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/core/applications/metrics"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/discovery/cache"
	"github.com/telark/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	"github.com/telark/kcore/resources/workload"
	"github.com/telark/rest/response"
)

// applyHistoryFromDiff returns, per application, whether the diff authored the
// history and snapshots it produced. Callers without an informer-captured
// pre-image echo back stored values, which must not be republished.
func applyHistoryFromDiff(
	ctx context.Context,
	apps *[]application.Application,
	opts GetApplicationsOptions,
) []bool {
	authored := make([]bool, len(*apps))
	for i := range *apps {
		app := &(*apps)[i]
		h, snaps, ok := historyAndSnapshotsForApp(ctx, app, opts)
		app.History = h
		app.Snapshots = snaps
		authored[i] = ok
		snapshot.NormalizeApplicationSnapshotTakenAt(app)
	}
	return authored
}

func historyAndSnapshotsForApp(
	ctx context.Context,
	app *application.Application,
	opts GetApplicationsOptions,
) (h application.ApplicationHistory, snaps []application.ApplicationSnapshot, authored bool) {
	defer func() {
		if r := recover(); r != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(string(constants.ErrApplicationHistoryDiffPanic), app.Name, r),
			)
			h = diff.NewApplicationHistory()
			snaps = []application.ApplicationSnapshot{}
			authored = false
		}
	}()
	var stored *application.Application
	if opts.GetStoredApplication != nil {
		stored = opts.GetStoredApplication(app.Name)
	}
	dopts := buildDiffOpts(app, opts)
	h, snaps, authored = diff.DiffApplications(
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
	return h, snaps, authored
}

func buildDiffOpts(app *application.Application, opts GetApplicationsOptions) *diff.DiffOptions {
	hasPrewritten := isPrewrittenForApp(app.Name, opts)
	if !opts.FromCoalescingFlush && !opts.FromForceSync && !hasPrewritten {
		return nil
	}
	dopts := &diff.DiffOptions{
		FromCoalescingFlush: opts.FromCoalescingFlush,
		FromForceSync:       opts.FromForceSync,
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
	enqueuedCount := constants.DefaultInitValue
	for i := range apps {
		apps[i].Health = ComputeHealth(&apps[i])
		if opts.InsightsEnabled {
			if attachEnrichment(ctx, rdb, &apps[i]) {
				enqueuedCount++
			}
		} else {
			apps[i].Insights = emptyInsights()
		}
		totalResources += apps[i].ResourceCount
	}

	authored := applyHistoryFromDiff(ctx, &apps, opts)
	for i := range apps {
		metrics.PopulateApplicationMetrics(ctx, &apps[i])
	}

	PublishApplications(opts.NatsClient, apps, authored)
	runEnrichmentWaitLoop(ctx, rdb, opts, enqueuedCount, apps)

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

func countUnenriched(apps []application.Application) int {
	n := constants.DefaultInitValue
	for i := range apps {
		if !apps[i].Insights.Enriched {
			n++
		}
	}
	return n
}

func refillEnrichment(ctx context.Context, rdb *redis.Client, apps []application.Application) {
	for i := range apps {
		if apps[i].Insights.Enriched {
			continue
		}
		primaryNamespace := constants.EmptyString
		if len(apps[i].Namespaces.Items) >= sortOne {
			primaryNamespace = apps[i].Namespaces.Items[firstItemIdx].Name
		}
		insights, err := cache.GetEnrichment(ctx, rdb, primaryNamespace, apps[i].Name)
		if err != nil || insights == nil || cache.IsStale(insights, apps[i].LastUpdated) {
			continue
		}
		apps[i].Insights = *insights
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
		resources := byApp[name]
		var stored *application.Application
		if opts.GetStoredApplication != nil {
			stored = opts.GetStoredApplication(name)
		}
		out = append(out, buildApplication(name, resources, stored))
	}
	return out
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
	key := r.Namespace + "/" + r.Kind + "/" + r.Name
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
	primaryNS := discoveryshared.PrimaryNamespaceFromCounts(agg.nsCounts)
	displayName := discoveryshared.BuildDisplayName(name, primaryNS)
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
		Insights:        application.Insights{},
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

// loads insights from Redis and enqueues if missing/stale
func attachEnrichment(ctx context.Context, rdb *redis.Client, app *application.Application) bool {
	primaryNamespace := constants.EmptyString
	if len(app.Namespaces.Items) >= sortOne {
		primaryNamespace = app.Namespaces.Items[firstItemIdx].Name
	}
	insights, err := cache.GetEnrichment(ctx, rdb, primaryNamespace, app.Name)
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnEnrichmentCacheReadFailed),
				app.Name, primaryNamespace, err),
		)
		app.Insights = emptyInsights()
		return false
	}
	if insights != nil && !cache.IsStale(insights, app.LastUpdated) {
		app.Insights = *insights
		return false
	}
	app.Insights = emptyInsights()
	if !cache.IsEnqueued(ctx, rdb, primaryNamespace, app.Name) {
		enqueueJobAsync(ctx, rdb, app)
		return true
	}
	return false
}

func enqueueJobAsync(ctx context.Context, rdb *redis.Client, app *application.Application) {
	go func(a application.Application) {
		if err := cache.EnqueueJob(ctx, rdb, a); err != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(msgEnqueueFailed, a.Name, err))
		}
	}(*app)
}

func emptyInsights() application.Insights {
	return application.Insights{
		Enriched:      false,
		EnrichedAt:    nil,
		Confidence:    nil,
		Summary:       nil,
		TechStack:     []string{},
		Role:          nil,
		Dependencies:  []string{},
		Category:      nil,
		Risks:         []string{},
		Suggestions:   []string{},
		RelatedApps:   []application.RelatedApp{},
		PromptVersion: nil,
	}
}

func runEnrichmentWaitLoop(
	ctx context.Context,
	rdb *redis.Client,
	opts GetApplicationsOptions,
	enqueuedCount int,
	apps []application.Application,
) {
	if !opts.InsightsEnabled {
		return
	}
	if !opts.Wait || enqueuedCount <= constants.DefaultInitValue {
		return
	}
	if countUnenriched(apps) == constants.DefaultInitValue {
		return
	}

	timeoutSec := opts.WaitTimeoutSec
	if timeoutSec <= constants.DefaultInitValue {
		timeoutSec = constants.WaitTimeoutDefaultSec
	}
	if timeoutSec > constants.WaitTimeoutMaxSec {
		timeoutSec = constants.WaitTimeoutMaxSec
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(waitPollIntervalSec * time.Second)
		refillEnrichment(ctx, rdb, apps)
		if countUnenriched(apps) == constants.DefaultInitValue {
			break
		}
	}
}
