package startup

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/kcore/resources/server"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

func PatchClusterVersionAsync(ctx context.Context) {
	go func() {
		lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
		lg.Info(string(constants.InfoClusterVersionPatchStarting))
		for attempt := constants.DefaultAddValue; attempt <= constants.ClusterVersionPatchMaxAttempts; attempt++ {
			if ctx.Err() != nil {
				return
			}

			err := doPatchClusterVersion(ctx, lg)
			if err == nil {
				return
			}

			if attempt == constants.ClusterVersionPatchMaxAttempts {
				lg.Error(fmt.Sprintf(string(constants.WarnClusterVersionPatchExhausted),
					constants.ClusterVersionPatchMaxAttempts, err))
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(constants.ClusterVersionPatchRetryBackoff):
			}
		}
	}()
}

func doPatchClusterVersion(ctx context.Context, lg interface {
	Info(string)
	Warn(string)
	Error(string)
},
) error {
	if !WaitForGlobalConfigReady(ctx) {
		return ctx.Err()
	}
	ver, err := server.GetServerVersion()
	if err != nil || ver == nil {
		return fmt.Errorf(string(constants.WarnClusterVersionPatchFailed), err)
	}

	resp := gcfgclient.NewClient().PatchGlobalConfig(map[string]any{
		"cluster": map[string]any{
			"version": ver.GitVersion,
		},
	})
	if resp == nil || resp.Status != http.StatusOK {
		return fmt.Errorf(string(constants.WarnClusterVersionPatchFailedStatus), resp)
	}

	lg.Info(fmt.Sprintf(string(constants.InfoClusterVersionPatched), ver.GitVersion))
	return nil
}
