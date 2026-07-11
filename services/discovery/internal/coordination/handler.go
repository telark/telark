package coordination

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	serviceapp "github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/discovery/listing"
	"github.com/telark/discovery/internal/discovery/prewarm"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
)

var errUnexpectedResponseData = errors.New("unexpected response data type")

// CancelCoalesceFn cancels any pending coalescer state for the given application
// before a force sync runs. Wired from main.go.
var CancelCoalesceFn func(appName string)

func executeHandler(
	ctx context.Context,
	rdb *redis.Client,
	namespace string,
	operation string,
) error {
	switch operation {
	case constants.OperationTypePrewarm:
		return executePrewarmHandler(ctx, rdb, namespace)
	default:
		return fmt.Errorf(string(constants.ErrUnknownOperation), operation)
	}
}

func RunPrewarmForNamespace(ctx context.Context, rdb *redis.Client, namespace string) error {
	return executePrewarmHandler(ctx, rdb, namespace)
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

	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
	if isStoredAppExcluded(stored, excluded) {
		return nil
	}

	namespaces, err := collectSyncNamespaces(stored, excluded)
	if err != nil {
		return err
	}

	resources, err := listing.Resources(ctx, namespaces)
	if err != nil {
		return err
	}
	inputs := discoveryshared.ToDerivationInputs(resources)
	syncOpts := prewarm.BuildPrewarmApplicationOptions()
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
	for i := range data.Applications {
		if data.Applications[i].Name == appName {
			return true
		}
	}
	return false
}

// RunForceSyncJob is the JobExecutor used by the force-sync worker pool. It
// acquires the per-app lock, runs the heartbeat, and invokes the sync handler.
func RunForceSyncJob(
	ctx context.Context,
	coord *CoordinationBundle,
	rdb *redis.Client,
	replicaID string,
	appName string,
) error {
	lockKey := constants.KeyPrefixLockApp + appName
	lockValue := replicaID + constants.ColonSeparator + appName

	acquired, err := coord.Lock.Acquire(ctx, lockKey, lockValue, coord.Config.LockTTL)
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

func executePrewarmHandler(ctx context.Context, rdb *redis.Client, namespace string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if isNamespaceExcluded(namespace, gcfghelper.FetchExcludedNamespaces(ctx)) {
		return nil
	}

	namespaces := []string{namespace}
	resources, err := listing.Resources(ctx, namespaces)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	inputs := discoveryshared.ToDerivationInputs(resources)
	opts := prewarm.BuildPrewarmApplicationOptions()
	resp := serviceapp.GetApplications(ctx, rdb, inputs, opts)

	if _, ok := resp.Data.(applicationmodel.ResponseData); !ok {
		return errUnexpectedResponseData
	}
	return nil
}

func isNamespaceExcluded(ns string, excluded []string) bool {
	return slices.Contains(excluded, ns)
}
