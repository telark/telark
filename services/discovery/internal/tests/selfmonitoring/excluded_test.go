package selfmonitoring

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/handlers/cleanup/autoclean"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	ownNS      = "telark"
	kubeSystem = "kube-system"
	appNS      = "shop"
)

func appIn(namespaces ...string) *appresource.Application {
	app := &appresource.Application{}
	for _, ns := range namespaces {
		app.Namespaces.Items = append(app.Namespaces.Items, appresource.NamespaceEntry{Name: ns})
	}
	return app
}

// Telark's own services, redis, nats and policy engine were discovered as applications; they
// stay hidden unless the operator opts into self-monitoring, and plans never target them.
func TestReleaseNamespaceHiddenUnlessSelfMonitoring(t *testing.T) {
	t.Setenv(constants.EnvPodNamespace, ownNS)
	tcfghelper.SetExcludedForTest([]string{kubeSystem})
	ctx := context.Background()
	platform, mixed := appIn(ownNS), appIn(ownNS, appNS)
	reserved := validation.PlatformNamespaces([]string{ownNS}, ownNS).Error()

	for _, selfMonitoring := range []bool{false, true} {
		t.Run("selfMonitoring="+strconv.FormatBool(selfMonitoring), func(t *testing.T) {
			t.Setenv(constants.EnvSelfMonitoringEnabled, strconv.FormatBool(selfMonitoring))
			excluded, err := tcfghelper.ExcludedNamespaces(ctx)
			if err != nil {
				t.Fatal(err)
			}
			testutil.Equal(t, "config list kept", slices.Contains(excluded, kubeSystem), true)
			testutil.Equal(t, "release namespace excluded", slices.Contains(excluded, ownNS), !selfMonitoring)
			testutil.Equal(t, "informers see the same list",
				slices.Contains(tcfghelper.FetchExcludedNamespaces(ctx), ownNS), !selfMonitoring)

			testutil.Equal(t, "platform app purged", autoclean.HiddenPlatformApp(platform), !selfMonitoring)
			testutil.Equal(t, "app outside the release namespace kept", autoclean.HiddenPlatformApp(mixed), false)

			err = validation.NamespaceScope(ctx, plans.ScopeTypeNamespaces, []string{ownNS}, nil)
			testutil.Equal(t, "plan on release namespace rejected", validation.IsValidation(err), true)
			testutil.Equal(t, "reserved message", err.Error(), reserved)
			testutil.Equal(t, "plan on app namespace",
				validation.NamespaceScope(ctx, plans.ScopeTypeNamespaces, []string{appNS}, nil), nil)
		})
	}
}
