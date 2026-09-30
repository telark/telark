package coordination

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	serviceapp "github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	"github.com/telark/telark/services/discovery/internal/discovery/prewarm"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
)

var errUnexpectedResponseData = errors.New("unexpected response data type")

// CancelCoalesceFn cancels any pending coalescer state for the given application
// before a force sync runs. Wired from main.go.
var CancelCoalesceFn func(appName string)

func executeHandler(
	ctx context.Context,
	rdb *redis.Client,
	namespace string,
	appName string,
	operation string,
) error {
	switch operation {
	case constants.OperationTypePrewarm:
		return executePrewarmHandler(ctx, rdb, namespace, appName)
	default:
		return fmt.Errorf(string(constants.ErrUnknownOperation), operation)
	}
}

func RunPrewarmForNamespace(ctx context.Context, rdb *redis.Client, namespace string) error {
	return executePrewarmHandler(ctx, rdb, namespace, constants.EmptyString)
}

func executeSyncHandler(ctx context.Context, rdb *redis.Client, appName string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if strings.TrimSpace(appName) == constants.EmptyString {
		return errors.New(string(constants.ErrAppNameRequired))
	}

	if CancelCoalesceFn != nil {
		CancelCoalesceFn(appName)
	}

	stored, err := clients.NewExporterClient().GetApplicationByName(appName)
	if err != nil || stored == nil {
		return fmt.Errorf(string(constants.ErrStoredApplicationNotFound), err)
	}

	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	if isStoredAppExcluded(stored, excluded) {
		return nil
	}

	namespaces, err := collectSyncNamespaces(stored, excluded)
	if err != nil {
		return err
	}

	resources, err := listing.Resources(ctx, listing.AppNamespaces(ctx, appName, namespaces))
	if err != nil {
		return err
	}
	inputs := discoveryshared.ToDerivationInputs(resources)
	// Per-app options drop the namespace's other apps at buildApplications;
	// the full-list variant re-diffed and re-published every neighbor per job.
	syncOpts := prewarm.BuildPrewarmApplicationOptionsForApp(appName)
	syncOpts.FromForceSync = true
	resp := serviceapp.GetApplications(ctx, rdb, inputs, syncOpts)
	data, ok := resp.Data.(applicationmodel.ResponseData)
	if !ok {
		return errUnexpectedResponseData
	}
	if !containsApp(data, appName) {
		return errors.New(string(constants.ErrApplicationNotFoundInComputedSet))
	}
	return nil
}

func isStoredAppExcluded(stored *applicationmodel.Application, excluded []string) bool {
	if len(stored.Namespaces.Items) == constants.DefaultInitValue {
		return false
	}
	for i := range stored.Namespaces.Items {
		if !isNamespaceExcluded(stored.Namespaces.Items[i].Name, excluded) {
			return false
		}
	}
	return true
}

func collectSyncNamespaces(stored *applicationmodel.Application, excluded []string) ([]string, error) {
	out := make([]string, constants.DefaultInitValue, len(stored.Namespaces.Items))
	for i := range stored.Namespaces.Items {
		ns := strings.TrimSpace(stored.Namespaces.Items[i].Name)
		if ns == constants.EmptyString {
			continue
		}
		if slices.Contains(excluded, ns) {
			continue
		}
		out = append(out, ns)
	}
	if len(out) == constants.DefaultInitValue {
		return nil, errors.New(string(constants.ErrNoNamespacesFoundForApplication))
	}
	return out, nil
}

func containsApp(data applicationmodel.ResponseData, appName string) bool {
	return slices.ContainsFunc(data.Applications, func(a applicationmodel.Application) bool {
		return a.Name == appName
	})
}

func RunForceSyncJob(
	ctx context.Context,
	coord *CoordinationBundle,
	rdb *redis.Client,
	replicaID string,
	appName string,
) error {
	lockKey := constants.KeyPrefixLockApp + appName
	lockValue := replicaID + constants.ColonSeparator + appName

	acquired, err := acquireLockWithWait(ctx, coord, lockKey, lockValue)
	if err != nil || !acquired {
		return fmt.Errorf(string(constants.ErrForceSyncLockNotAcquired), appName)
	}

	hbCtx, cancelHB := context.WithCancel(ctx)
	defer cancelHB()
	go runLockHeartbeat(hbCtx, cancelHB, coord, lockKey, lockValue, appName)

	handlerErr := executeSyncHandler(ctx, rdb, appName)

	_ = coord.Lock.Release(context.Background(), lockKey, lockValue)
	return handlerErr
}

// The informer consumer holds the same per-app lock while it processes an app;
// a force sync that lands in that window waits its turn instead of failing.
func acquireLockWithWait(
	ctx context.Context,
	coord *CoordinationBundle,
	lockKey, lockValue string,
) (bool, error) {
	for {
		acquired, err := coord.Lock.Acquire(ctx, lockKey, lockValue, coord.Config.LockTTL)
		if err != nil || acquired {
			return acquired, err
		}
		select {
		case <-ctx.Done():
			return false, nil
		case <-time.After(constants.ForceSyncLockRetryInterval):
		}
	}
}

func executePrewarmHandler(ctx context.Context, rdb *redis.Client, namespace string, appName string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if isNamespaceExcluded(namespace, tcfghelper.FetchExcludedNamespaces(ctx)) {
		return nil
	}

	namespaces := []string{namespace}
	if appName != constants.EmptyString {
		namespaces = listing.AppNamespaces(ctx, appName, namespaces)
	}
	resources, err := listing.Resources(ctx, namespaces)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	inputs := discoveryshared.ToDerivationInputs(resources)
	opts := prewarm.BuildPrewarmApplicationOptions()
	if appName != constants.EmptyString {
		opts = prewarm.BuildPrewarmApplicationOptionsForApp(appName)
	} else {
		opts.GetStoredApplication = storedIfOnlyIn(ctx, namespace, opts.GetStoredApplication)
	}
	resp := serviceapp.GetApplications(ctx, rdb, inputs, opts)

	if _, ok := resp.Data.(applicationmodel.ResponseData); !ok {
		return errUnexpectedResponseData
	}
	return nil
}

// An app with objects in other namespaces is left to its per-app job, which lists
// them all; decided once per app so the build and diff lookups cannot disagree.
func storedIfOnlyIn(
	ctx context.Context,
	namespace string,
	lookup func(string) (*applicationmodel.Application, error),
) func(string) (*applicationmodel.Application, error) {
	spans := make(map[string]bool)
	return func(name string) (*applicationmodel.Application, error) {
		elsewhere, decided := spans[name]
		if !decided {
			elsewhere = len(listing.AppNamespaces(ctx, name, []string{namespace})) > constants.DefaultAddValue
			spans[name] = elsewhere
		}
		if elsewhere {
			return nil, serviceapp.ErrNotJobTarget
		}
		return lookup(name)
	}
}

func isNamespaceExcluded(ns string, excluded []string) bool {
	return slices.Contains(excluded, ns)
}
