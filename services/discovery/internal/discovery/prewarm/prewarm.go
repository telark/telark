package prewarm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/kcore/resources/core"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	serviceapp "github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	natshelper "github.com/telark/telark/services/discovery/internal/helpers/nats"
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
	lg.Info(string(constants.InfoPrewarmCacheStarted))
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
	opts := serviceapp.GetApplicationsOptions{NatsClient: natshelper.GetClient()}

	exporterClient := clients.NewExporterClient()
	snapshotClient := clients.NewSnapshotClient()
	opts.GetStoredApplication = storedApplicationLookup(exporterClient)
	opts.CreateSnapshot = func(id, scope, namespace string, generation int, manifest any) (string, error) {
		return snapshotClient.CreateSnapshotAndReturnPath(id, scope, namespace, generation, manifest)
	}
	opts.DeleteSnapshot = func(id, scope, namespace string, generation int) error {
		return snapshotClient.DeleteSnapshot(id, scope, namespace, generation)
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

// Only appName is looked up (one GET instead of the full list) and published; the rest of the
// namespace is derived for grouping only. A missing app is a new one.
func BuildPrewarmApplicationOptionsForApp(appName string) serviceapp.GetApplicationsOptions {
	opts := BuildPrewarmApplicationOptions()
	exporterClient := clients.NewExporterClient()
	opts.GetStoredApplication = func(name string) (*applicationmodel.Application, error) {
		if name != appName {
			return nil, serviceapp.ErrNotJobTarget
		}
		stored, err := exporterClient.GetApplicationByName(name)
		if errors.Is(err, restshared.ErrNotFound) {
			return nil, nil
		}
		return stored, err
	}
	return opts
}

// Listed once per derivation: a name missing from a successful list is a new application, while
// a failed list makes every application unknown, which callers must treat as "do not publish".
func storedApplicationLookup(exporterClient *clients.ExporterClient) func(string) (*applicationmodel.Application, error) {
	var once sync.Once
	var byName map[string]*applicationmodel.Application
	var listErr error
	return func(name string) (*applicationmodel.Application, error) {
		once.Do(func() {
			apps, err := exporterClient.GetAllApplications()
			if err != nil {
				listErr = err
				return
			}
			byName = make(map[string]*applicationmodel.Application, len(apps))
			for _, app := range apps {
				if app != nil {
					byName[app.Name] = app
				}
			}
		})
		if listErr != nil {
			return nil, listErr
		}
		return byName[name], nil
	}
}
