package globalconfig

import (
	"context"

	"github.com/telark/discovery/startup"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

func FetchExcludedNamespaces(ctx context.Context) []string {
	if v := excludedCache.Load(); v != nil {
		if out, ok := v.([]string); ok {
			return out
		}
	}
	if !startup.WaitForGlobalConfigReady(ctx) {
		return nil
	}
	cfg, err := gcfgclient.NewClient().GetGlobalConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.ExcludedNamespaces
}
