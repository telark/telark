package telarkconfig

import (
	"context"
	"fmt"
	"os"
	"slices"

	cfgclient "github.com/telark/telark/internal/rest/clients/config"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/startup"
)

// The last successful read wins over a failed refresh; only a list that was
// never loaded is reported as unavailable, so callers can fail closed.
func ExcludedNamespaces(ctx context.Context) ([]string, error) {
	if out, ok := excludedCache.Load().([]string); ok {
		return withHiddenOwnNamespace(out), nil
	}
	if !startup.WaitForTelarkConfigReady(ctx) {
		return nil, fmt.Errorf(string(constants.ErrExcludedNamespacesUnavailable), ctx.Err())
	}
	cfg, err := cfgclient.NewClient().GetConfig()
	if err != nil || cfg == nil {
		return nil, fmt.Errorf(string(constants.ErrExcludedNamespacesUnavailable), err)
	}
	excludedCache.Store(cfg.ExcludedNamespaces)
	return withHiddenOwnNamespace(cfg.ExcludedNamespaces), nil
}

// Informer and rail call sites keep treating an unknown list as empty; the
// derive, prewarm and force-sync passes go through ExcludedNamespaces instead.
func FetchExcludedNamespaces(ctx context.Context) []string {
	out, _ := ExcludedNamespaces(ctx)
	return out
}

func withHiddenOwnNamespace(excluded []string) []string {
	own := HiddenOwnNamespace()
	if own == constants.EmptyString || slices.Contains(excluded, own) {
		return excluded
	}
	return append(slices.Clone(excluded), own)
}

// Telark's own components (its services, redis, nats, ollama, the policy engine) are not
// applications unless the operator opts into self-monitoring.
func HiddenOwnNamespace() string {
	if config.SelfMonitoringEnabled() {
		return constants.EmptyString
	}
	return OwnNamespace()
}

// Empty outside a cluster, which disables every own-namespace check rather than failing them.
func OwnNamespace() string {
	return os.Getenv(constants.EnvPodNamespace)
}

// Test seam: installs the list the sync loop would otherwise load from TelarkConfig.
func SetExcludedForTest(namespaces []string) {
	excludedCache.Store(namespaces)
}
