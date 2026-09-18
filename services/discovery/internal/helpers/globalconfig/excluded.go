package globalconfig

import (
	"context"
	"fmt"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/startup"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

// The last successful read wins over a failed refresh; only a list that was
// never loaded is reported as unavailable, so callers can fail closed.
func ExcludedNamespaces(ctx context.Context) ([]string, error) {
	if out, ok := excludedCache.Load().([]string); ok {
		return out, nil
	}
	if !startup.WaitForGlobalConfigReady(ctx) {
		return nil, fmt.Errorf(string(constants.ErrExcludedNamespacesUnavailable), ctx.Err())
	}
	cfg, err := gcfgclient.NewClient().GetGlobalConfig()
	if err != nil || cfg == nil {
		return nil, fmt.Errorf(string(constants.ErrExcludedNamespacesUnavailable), err)
	}
	excludedCache.Store(cfg.ExcludedNamespaces)
	return cfg.ExcludedNamespaces, nil
}

// Informer and rail call sites keep treating an unknown list as empty; the
// derive, prewarm and force-sync passes go through ExcludedNamespaces instead.
func FetchExcludedNamespaces(ctx context.Context) []string {
	out, _ := ExcludedNamespaces(ctx)
	return out
}
