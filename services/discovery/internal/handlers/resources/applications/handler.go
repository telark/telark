package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/coordination"
	serviceapp "github.com/telark/discovery/core/applications/core"
	"github.com/telark/discovery/discovery/listing"
	discoveryshared "github.com/telark/discovery/discovery/shared"
	natshelper "github.com/telark/discovery/helpers/nats"
	redishelper "github.com/telark/discovery/helpers/redis"
	sharedhelper "github.com/telark/discovery/helpers/shared"
	kcoregroup "github.com/telark/kcore/resources/group"
	"github.com/telark/rest/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EnrichApplications returns the full applications (grouping + enrichment).
func EnrichApplications(w http.ResponseWriter, r *http.Request) {
	selector, selectorOK := parseEnrichmentSelectorParams(r)
	if !selectorOK {
		sendApplicationsError(w, http.StatusBadRequest, string(constants.ErrSelectorTypeMustBeLabelsOrText))
		return
	}

	namespace, _ := sharedhelper.GetOptionalQueryParam(r, constants.NamespaceParamQuery)
	namespaces := discoveryshared.ParseNamespaceList(namespace)

	ctx, cancel := context.WithTimeout(context.Background(), constants.EnrichApplicationsTimeout)
	defer cancel()

	resources, err := listOrSearchResources(ctx, namespaces, selector)
	if err != nil {
		sendApplicationsError(w, http.StatusInternalServerError, string(constants.ErrInternalServerError))
		return
	}

	inputs := discoveryshared.ToDerivationInputs(resources)
	rdb := redishelper.NewRedisClient()

	enrichCtx, cleanup, conflict := acquireEnrichLock(ctx, selector, namespace)
	if conflict {
		sendApplicationsError(w, http.StatusConflict,
			"Enrichment already in progress for this application.")
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	opts := buildEnrichmentApplicationOptions(r)
	resp := serviceapp.GetApplications(enrichCtx, rdb, inputs, opts)

	if enrichCtx.Err() != nil && cleanup != nil {
		sendApplicationsError(w, http.StatusInternalServerError,
			"Enrichment aborted — lock lost.")
		return
	}

	if selector == constants.EmptyString {
		resp.Message = string(constants.MessageApplicationsListAll)
	}
	writeApplicationsJSON(w, resp)
}

func acquireEnrichLock(
	ctx context.Context,
	selector, namespace string,
) (context.Context, func(), bool) {
	coord, replicaID := getCoordinationBundle()
	if coord == nil {
		return ctx, nil, false
	}

	appID := resolveAppIdentifier(selector, namespace)
	lockKey := constants.KeyPrefixLockEnrich + appID
	lockValue := replicaID + ":" + appID

	acquired, lockErr := coord.Lock.Acquire(ctx, lockKey, lockValue, coord.Config.LockTTL)
	if lockErr != nil || !acquired {
		lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
		lg.Error(fmt.Sprintf(string(constants.ErrEnrichLockNotAcquired), appID))
		return ctx, nil, true
	}

	hbCtx, cancelHB := context.WithCancel(ctx)
	go runEnrichLockHeartbeat(hbCtx, cancelHB, coord, lockKey, lockValue, appID)

	cleanup := func() {
		cancelHB()
		_ = coord.Lock.Release(context.Background(), lockKey, lockValue)
		lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
		lg.Info(fmt.Sprintf(string(constants.LogEnrichComplete), appID))
	}
	return hbCtx, cleanup, false
}

func resolveAppIdentifier(selector, namespace string) string {
	if selector != constants.EmptyString {
		return selector
	}
	if namespace != constants.EmptyString {
		return namespace
	}
	return "all"
}

func runEnrichLockHeartbeat(
	ctx context.Context,
	cancelHB context.CancelFunc,
	coord *coordination.CoordinationBundle,
	lockKey, lockValue, appName string,
) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	ticker := time.NewTicker(coord.Config.LockHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ok, err := coord.Lock.Extend(
				ctx, lockKey, lockValue, coord.Config.LockTTL,
			)
			if !ok || err != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrEnrichLockLost), appName))
				cancelHB()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func parseEnrichmentSelectorParams(r *http.Request) (selector string, ok bool) {
	rawSelector, _ := sharedhelper.GetOptionalQueryParam(r, constants.SelectorParam)
	selector = strings.TrimSpace(rawSelector)

	st, _ := sharedhelper.GetOptionalQueryParam(r, "selectorType")
	selectorType := strings.TrimSpace(strings.ToLower(st))
	if selector != constants.EmptyString {
		if selectorType == constants.EmptyString {
			selectorType = constants.SelectorTypeLabels
		}
		if selectorType != constants.SelectorTypeLabels && selectorType != constants.SelectorTypeText {
			return selector, false
		}
	}
	return selector, true
}

func listOrSearchResources(
	ctx context.Context,
	namespaces []string,
	selector string,
) ([]kcoregroup.ResourceRef, error) {
	if selector == constants.EmptyString {
		return listing.Resources(ctx, namespaces)
	}
	return kcoregroup.SearchResourcesByLabelOrTextInNamespaces(selector, namespaces)
}

func buildEnrichmentApplicationOptions(r *http.Request) serviceapp.GetApplicationsOptions {
	opts := getApplicationsOptions(r)
	insightsParam, _ := sharedhelper.GetOptionalQueryParam(r, "insights")
	opts.InsightsEnabled = strings.ToLower(strings.TrimSpace(insightsParam)) == "enabled"
	if nc, err := natshelper.GetClient(); err == nil && nc != nil {
		opts.NatsClient = nc
	}

	exporterClient := clients.NewExporterClient()
	snapshotClient := clients.NewSnapshotClient()
	defaultSnapshotScope := config.DefaultSnapshotScope()
	opts.GetStoredApplication = func(name string) *applicationmodel.Application {
		app, err := exporterClient.GetApplicationByName(name)
		if err != nil {
			return nil
		}
		return app
	}
	opts.CreateSnapshot = func(id, scope, namespace string, generation int, manifest any) (string, error) {
		useScope := scope
		if strings.TrimSpace(useScope) == constants.EmptyString {
			useScope = defaultSnapshotScope
		}
		return snapshotClient.CreateSnapshotAndReturnPath(id, useScope, namespace, generation, manifest)
	}
	opts.GetSnapshotManifest = func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error) {
		useScope := scope
		if strings.TrimSpace(useScope) == constants.EmptyString {
			useScope = defaultSnapshotScope
		}
		return snapshotClient.GetSnapshotManifest(ctx, snapshotID, useScope, namespace, generation)
	}
	return opts
}

func writeApplicationsJSON(w http.ResponseWriter, resp response.GenericResponse) {
	if d, ok := resp.Data.(applicationmodel.ResponseData); ok {
		resp.Data = d
	}
	w.Header().Set("Content-Type", constants.ApplicationJSON)
	w.WriteHeader(resp.Status)
	_ = json.NewEncoder(w).Encode(resp)
}

func getApplicationsOptions(r *http.Request) serviceapp.GetApplicationsOptions {
	opts := serviceapp.GetApplicationsOptions{}
	waitVal, ok := sharedhelper.GetOptionalQueryParam(r, "wait")
	opts.Wait = ok && strings.ToLower(strings.TrimSpace(waitVal)) == "true"
	timeoutVal, ok := sharedhelper.GetOptionalQueryParam(r, "waitTimeout")
	if !ok || timeoutVal == "" {
		opts.WaitTimeoutSec = constants.WaitTimeoutDefaultSec
		return opts
	}
	sec, err := strconv.Atoi(strings.TrimSpace(timeoutVal))
	if err != nil || sec <= constants.DefaultInitValue {
		opts.WaitTimeoutSec = constants.WaitTimeoutDefaultSec
		return opts
	}
	if sec > constants.WaitTimeoutMaxSec {
		sec = constants.WaitTimeoutMaxSec
	}
	opts.WaitTimeoutSec = sec
	return opts
}

func sendApplicationsError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", constants.ApplicationJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response.GenericResponse{
		Status:    status,
		Operation: string(response.OperationError),
		Message:   message,
		Data:      applicationmodel.ResponseData{Applications: []applicationmodel.Application{}},
	})
}
