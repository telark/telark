package selfmonitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/plans"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/config"
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

	platformApp = "telark-redis"
	mixedApp    = "shop-with-cache"
	emptyApp    = "shop-gone"

	cycleInterval = 10 * time.Millisecond
	cyclesNeeded  = 1000
	cycleDeadline = 5 * time.Second
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func appIn(name string, resources int, namespaces ...string) *appresource.Application {
	app := &appresource.Application{Name: name, ResourceCount: resources}
	for _, ns := range namespaces {
		app.Namespaces.Items = append(app.Namespaces.Items, appresource.NamespaceEntry{Name: ns})
	}
	return app
}

// Runs the auto-cleanup detector over the stored apps until a cycle reaches the last one, an app
// with nothing left, and reports which apps it started purging (an empty streak opened).
func purgeStarted(t *testing.T, apps ...*appresource.Application) map[string]bool {
	t.Helper()
	body, err := json.Marshal(map[string]any{"data": map[string]any{"items": apps}})
	if err != nil {
		t.Fatal(err)
	}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	detector := autoclean.NewDetector(config.AutoCleanupConfig{
		CycleInterval: cycleInterval, EmptyCyclesRequired: cyclesNeeded, GracePeriod: time.Hour,
	}, rdb, clients.NewExporterClient(), nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		detector.Run(ctx)
		close(done)
	}()
	last := constants.KeyPrefixAutoCleanupEmptyStreak + apps[len(apps)-constants.DefaultAddValue].Name
	for deadline := time.Now().Add(cycleDeadline); !mr.Exists(last) && time.Now().Before(deadline); {
		time.Sleep(cycleInterval)
	}
	cancel()
	<-done
	started := make(map[string]bool, len(apps))
	for _, app := range apps {
		started[app.Name] = mr.Exists(constants.KeyPrefixAutoCleanupEmptyStreak + app.Name)
	}
	testutil.Equal(t, "a cycle reached the last app", started[emptyApp], true)
	return started
}

// Telark's own services, redis, nats and policy engine were discovered as applications; they
// stay hidden unless the operator opts into self-monitoring, and plans never target them.
func TestReleaseNamespaceHiddenUnlessSelfMonitoring(t *testing.T) {
	t.Setenv(constants.EnvPodNamespace, ownNS)
	testutil.ExcludedNamespaces(t, kubeSystem)
	ctx := context.Background()
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

			selectable := tcfghelper.SelectableNamespaces([]string{appNS, kubeSystem, ownNS})
			testutil.Equal(t, "release namespace selectable", slices.Contains(selectable, ownNS), selfMonitoring)
			testutil.Equal(t, "other namespaces selectable",
				slices.Contains(selectable, appNS) && slices.Contains(selectable, kubeSystem), true)

			started := purgeStarted(t, appIn(platformApp, constants.DefaultAddValue, ownNS),
				appIn(mixedApp, constants.DefaultAddValue, ownNS, appNS), appIn(emptyApp, constants.DefaultInitValue, appNS))
			testutil.Equal(t, "platform app purged", started[platformApp], !selfMonitoring)
			testutil.Equal(t, "app outside the release namespace kept", started[mixedApp], false)

			err = validation.NamespaceScope(ctx, plans.ScopeTypeNamespaces, []string{ownNS}, nil)
			testutil.Equal(t, "plan on release namespace rejected", validation.IsValidation(err), true)
			testutil.Equal(t, "reserved message", err.Error(), reserved)
			testutil.Equal(t, "plan on app namespace",
				validation.NamespaceScope(ctx, plans.ScopeTypeNamespaces, []string{appNS}, nil), nil)
		})
	}
}
