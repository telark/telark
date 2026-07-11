package prewarm

import (
	"context"
	"errors"
	"fmt"
	"time"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/constants"
	serviceapp "github.com/telark/discovery/core/applications/core"
	"github.com/telark/discovery/discovery/listing"
	discoveryshared "github.com/telark/discovery/discovery/shared"
	natshelper "github.com/telark/discovery/helpers/nats"
	"github.com/telark/kcore/resources/core"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	initialBackoff    = 2 * time.Second
	maxBackoff        = 30 * time.Second
	backoffMultiplier = 2
)

func PrewarmDiscovery(ctx context.Context, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	lg.Info(string(constants.InfoPrewarmEnrichmentStarted))
	runPrewarmWithSupervision(ctx, rdb)
}

func runPrewarmWithSupervision(ctx context.Context, rdb *redis.Client) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrorPrewarmRecoveredFromPanic), r))
		}
	}()

	backoff := initialBackoff
	for {
		if ctx.Err() != nil {
			lg.Info(string(constants.InfoPrewarmSupervisorShuttingDown))
			return
		}
		if err := runPrewarmOnce(ctx, rdb); err == nil {
			break
		}
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= backoffMultiplier
		}
	}
}

func runPrewarmOnce(ctx context.Context, rdb *redis.Client) (errOut error) {
	start := time.Now()
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	defer func() {
		if r := recover(); r != nil {
			errOut = fmt.Errorf(string(constants.ErrorPrewarmRecoveredFromPanic), r)
			lg.Error(fmt.Sprintf(string(constants.ErrorPrewarmRecoveredFromRunPanic), r))
		}
	}()

	nsList, err := core.GetAllNamespaces()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrPrewarmListNamespaces), err))
		return err
	}
	namespaces := make([]string, constants.DefaultInitValue, len(nsList))
	for i := range nsList {
		namespaces = append(namespaces, nsList[i].Name)
	}

	resources, err := listing.Resources(ctx, namespaces)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrPrewarmListResources), err))
		return err
	}

	inputs := discoveryshared.ToDerivationInputs(resources)

	opts := BuildPrewarmApplicationOptions()
	resp := serviceapp.GetApplications(ctx, rdb, inputs, opts)
	data, ok := resp.Data.(applicationmodel.ResponseData)
	if !ok {
		lg.Error(string(constants.ErrPrewarmUnexpectedResponseData))
		return errors.New(string(constants.ErrPrewarmUnexpectedResponseData))
	}
	apps := data.Applications

	nsSet := make(map[string]struct{})
	for _, app := range apps {
		for _, ns := range app.Namespaces.Items {
			nsSet[ns.Name] = struct{}{}
		}
	}
	lg.Info(fmt.Sprintf(string(constants.InfoPrewarmFoundApplications), len(apps), len(nsSet)))
	lg.Info(fmt.Sprintf(string(constants.InfoPrewarmComplete),
		len(apps), time.Since(start).Milliseconds()))
	return nil
}

func BuildPrewarmApplicationOptions() serviceapp.GetApplicationsOptions {
	opts := serviceapp.GetApplicationsOptions{
		InsightsEnabled: false,
	}
	if nc, err := natshelper.GetClient(); err == nil && nc != nil {
		opts.NatsClient = nc
	}

	exporterClient := clients.NewExporterClient()
	snapshotClient := clients.NewSnapshotClient()
	opts.GetStoredApplication = func(name string) *applicationmodel.Application {
		app, err := exporterClient.GetApplicationByName(name)
		if err != nil {
			return nil
		}
		return app
	}
	opts.CreateSnapshot = func(id, scope, namespace string, generation int, manifest any) (string, error) {
		return snapshotClient.CreateSnapshotAndReturnPath(id, scope, namespace, generation, manifest)
	}
	opts.GetSnapshotManifest = func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error) {
		return snapshotClient.GetSnapshotManifest(ctx, snapshotID, scope, namespace, generation)
	}
	return opts
}
