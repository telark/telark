package autoclean

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type railResult struct {
	name    string
	pass    bool
	reason  string // when pass=false, human-readable explanation for logs
	hardErr error  // set when the rail itself failed (Redis down, etc.) — caller skips streak update
}

func pass(name string) railResult { return railResult{name: name, pass: true} }
func block(name, reason string) railResult {
	return railResult{name: name, pass: false, reason: reason}
}

func errored(name string, err error) railResult {
	return railResult{name: name, pass: false, reason: err.Error(), hardErr: err}
}

func railResourcesEmpty(app *appresource.Application) railResult {
	if app == nil {
		return block(constants.RailResourcesEmpty, "app nil")
	}
	if app.ResourceCount != constants.DefaultInitValue {
		return block(constants.RailResourcesEmpty,
			fmt.Sprintf("ResourceCount=%d", app.ResourceCount))
	}
	if len(app.Resources) != constants.DefaultInitValue {
		return block(constants.RailResourcesEmpty,
			fmt.Sprintf("len(Resources)=%d", len(app.Resources)))
	}
	return pass(constants.RailResourcesEmpty)
}

func railNoActiveRollback(app *appresource.Application) railResult {
	if app == nil {
		return pass(constants.RailNoActiveRollback)
	}
	for i := range app.Rollbacks {
		s := app.Rollbacks[i].Status
		if s == constants.RollbackStatusPending || s == constants.RollbackStatusInProgress {
			return block(constants.RailNoActiveRollback,
				fmt.Sprintf("rollback %s status=%s", app.Rollbacks[i].ID, s))
		}
	}
	return pass(constants.RailNoActiveRollback)
}

func railNamespaceExists(ctx context.Context, kube *kubernetes.Clientset, app *appresource.Application) railResult {
	if app == nil || kube == nil {
		return pass(constants.RailNamespaceExists)
	}
	for i := range app.Namespaces.Items {
		ns := app.Namespaces.Items[i].Name
		got, err := kube.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err != nil {
			// A deleted namespace took the app's resources with it: nothing left to wait for.
			if k8serrors.IsNotFound(err) {
				continue
			}
			return errored(constants.RailNamespaceExists, err)
		}
		if got.DeletionTimestamp != nil {
			return block(constants.RailNamespaceExists,
				fmt.Sprintf("namespace %s is terminating", ns))
		}
	}
	return pass(constants.RailNamespaceExists)
}

// The informers stop with the namespace, so the app's stored resource list is
// stale and must be ignored once every namespace is gone.
func namespacesGone(ctx context.Context, kube *kubernetes.Clientset, app *appresource.Application) bool {
	if app == nil || kube == nil || len(app.Namespaces.Items) == constants.DefaultInitValue {
		return false
	}
	for i := range app.Namespaces.Items {
		_, err := kube.CoreV1().Namespaces().Get(ctx, app.Namespaces.Items[i].Name, metav1.GetOptions{})
		if !k8serrors.IsNotFound(err) {
			return false
		}
	}
	return true
}

func railNamespaceIncluded(ctx context.Context, app *appresource.Application) railResult {
	if app == nil || hiddenPlatformApp(app) {
		return pass(constants.RailNamespaceIncluded)
	}
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	if len(excluded) == constants.DefaultInitValue {
		return pass(constants.RailNamespaceIncluded)
	}
	exSet := make(map[string]struct{}, len(excluded))
	for _, e := range excluded {
		exSet[e] = struct{}{}
	}
	for i := range app.Namespaces.Items {
		ns := app.Namespaces.Items[i].Name
		if _, ok := exSet[ns]; ok {
			return block(constants.RailNamespaceIncluded,
				fmt.Sprintf("namespace %s is in excluded list", ns))
		}
	}
	return pass(constants.RailNamespaceIncluded)
}

// Hidden, not vanished: with self-monitoring off, Telark's own components keep their resources,
// so their stored CRs are purged without waiting for those to empty.
func hiddenPlatformApp(app *appresource.Application) bool {
	own := tcfghelper.HiddenOwnNamespace()
	return own != constants.EmptyString && len(app.Namespaces.Items) > constants.DefaultInitValue &&
		!slices.ContainsFunc(app.Namespaces.Items, func(n appresource.NamespaceEntry) bool { return n.Name != own })
}

func railRedisKeyAbsent(
	ctx context.Context,
	rdb redis.Cmdable,
	railName, key string,
) railResult {
	if rdb == nil {
		return pass(railName)
	}
	cctx, cancel := context.WithTimeout(ctx, constants.AutoCleanupRailReadTimeout)
	defer cancel()
	n, err := rdb.Exists(cctx, key).Result()
	if err != nil {
		return errored(railName, err)
	}
	if n > constants.DefaultInitValue {
		return block(railName, "redis key present: "+key)
	}
	return pass(railName)
}

func railNoForceSync(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	if r := railRedisKeyAbsent(ctx, rdb, constants.RailNoForceSync, constants.KeyPrefixLockApp+appName); !r.pass {
		return r
	}
	return railRedisKeyAbsent(ctx, rdb, constants.RailNoForceSync,
		constants.ForceSyncDedupKeyPrefix+appName)
}

func railNoCoalesceBuffer(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	return railRedisKeyAbsent(ctx, rdb, constants.RailNoCoalesceBuffer,
		constants.KeyPrefixCoalesceBuffer+appName)
}

// The analyzer keys its run lease by namespace too; the rail only has the app name.
func railNoAnalyzerInflight(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	if rdb == nil {
		return pass(constants.RailNoAnalyzerInflight)
	}
	cctx, cancel := context.WithTimeout(ctx, constants.AutoCleanupRailReadTimeout)
	defer cancel()
	pattern := constants.KeyPrefixAnalyzerInflight + "*:" + appName
	keys, _, err := rdb.Scan(cctx, 0, pattern, 10).Result()
	if err != nil {
		return errored(constants.RailNoAnalyzerInflight, err)
	}
	if len(keys) > constants.DefaultInitValue {
		return block(constants.RailNoAnalyzerInflight,
			fmt.Sprintf("analyzer run(s) in flight: %v", keys))
	}
	return pass(constants.RailNoAnalyzerInflight)
}

// Uses SCAN with pattern: cardinality per app is small (current gen only).
func railNoGenerationLock(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	if rdb == nil {
		return pass(constants.RailNoGenerationLock)
	}
	cctx, cancel := context.WithTimeout(ctx, constants.AutoCleanupRailReadTimeout)
	defer cancel()
	pattern := constants.KeyPrefixLockGen + appName + ":*"
	keys, _, err := rdb.Scan(cctx, 0, pattern, 10).Result()
	if err != nil {
		return errored(constants.RailNoGenerationLock, err)
	}
	if len(keys) > constants.DefaultInitValue {
		return block(constants.RailNoGenerationLock,
			fmt.Sprintf("generation lock(s) held: %v", keys))
	}
	return pass(constants.RailNoGenerationLock)
}

func railCleanupCooldown(ctx context.Context, rdb redis.Cmdable, appName string) railResult {
	return railRedisKeyAbsent(ctx, rdb, constants.RailCleanupCooldown,
		constants.KeyPrefixResetCooldown+appName)
}

func railSustainedAbsence(s streakState, required int) railResult {
	if s.Count < required {
		return block(constants.RailSustainedAbsence,
			fmt.Sprintf("emptyCycles=%d < required=%d", s.Count, required))
	}
	return pass(constants.RailSustainedAbsence)
}

func railGracePeriod(s streakState, grace time.Duration, now time.Time) railResult {
	if s.FirstEmptyUnixMs == int64(constants.DefaultInitValue) {
		return block(constants.RailGracePeriod, "first-empty timestamp unset")
	}
	first := time.UnixMilli(s.FirstEmptyUnixMs).UTC()
	if now.Sub(first) < grace {
		return block(constants.RailGracePeriod,
			fmt.Sprintf("elapsed=%s < grace=%s", now.Sub(first), grace))
	}
	return pass(constants.RailGracePeriod)
}
