package install

import (
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	keyWait      = "wait"
	keySecurity  = "security"
	keyBootstrap = "bootstrap"
	keyOld       = "breakGlass"
	keyHours     = "waitHours"
	keyAuth      = "auth"
	oldHours     = 48
	storedHours  = 12
	chartWait    = "24"
	valueMoved   = "✔ The release's app.security.breakGlass.waitHours (48) moves to app.auth.request.waitHours, its name since 0.3.0\n"
	valueDropped = "✔ The release's app.security.breakGlass is dropped: app.auth.request.waitHours replaces it since 0.3.0\n"
	oldKeyRefuse = "additional properties 'breakGlass' not allowed"
)

// A 0.2 release stored the key the 0.3.0 chart renamed, which no install of the test chart can do anymore.
func storeValues(t *testing.T, cluster *testutil.FakeCluster, app map[string]any) {
	t.Helper()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: cluster.URL})
	if err != nil {
		t.Fatal(err)
	}
	releases := driver.NewSecrets(client.CoreV1().Secrets(namespace))
	stored, err := releases.Get(firstRelease)
	if err != nil {
		t.Fatal(err)
	}
	release, ok := stored.(*releasev1.Release)
	if !ok {
		t.Fatalf("the stored release is a %T", stored)
	}
	release.Config = map[string]any{"app": app}
	if err := releases.Update(firstRelease, release); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeMovesTheRenamedValue(t *testing.T) {
	oldKey := map[string]any{keyOld: map[string]any{keyHours: oldHours}}
	bootstrap := map[string]any{keyBootstrap: map[string]any{"admin": adminEmail}}
	cases := map[string]valuesCase{
		"the old key alone": {app: map[string]any{keySecurity: oldKey, keyAuth: bootstrap}, line: valueMoved, wait: "48"},
		"the new key on the command line": {
			app: map[string]any{keySecurity: oldKey, keyAuth: bootstrap}, args: []string{setFlag, "app.auth.request.waitHours=72"},
			line: valueDropped, wait: "72",
		},
		"the new key in the release too": {
			app: map[string]any{keySecurity: oldKey, keyAuth: map[string]any{
				keyBootstrap: bootstrap[keyBootstrap], "request": map[string]any{keyHours: storedHours}}},
			line: valueDropped, wait: "12",
		},
		"an old block without the hours": {
			app:  map[string]any{keySecurity: map[string]any{keyOld: map[string]any{}}, keyAuth: bootstrap},
			line: valueDropped, wait: chartWait,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, previous)
			storeValues(t, cluster, c.app)
			dryRun := upgrade(t, kubeconfig, current, append([]string{dryRunFlag}, c.args...)...)
			testutil.Contains(t, stdoutLabel, dryRun.Stdout, c.line)
			testutil.Equal(t, codeLabel, dryRun.Code, passed)
			result := upgrade(t, kubeconfig, current, c.args...)
			testutil.Contains(t, stdoutLabel, result.Stdout, c.line)
			testutil.Equal(t, codeLabel, result.Code, passed)
			testutil.Equal(t, keyWait, data(cluster, keyWait), c.wait)
			testutil.Equal(t, "kept admin", data(cluster, "admin"), adminEmail)
			again := upgrade(t, kubeconfig, current)
			testutil.Equal(t, "moved once", strings.Contains(again.Stdout, keyOld), false)
			testutil.Equal(t, "kept after the move", data(cluster, keyWait), c.wait)
		})
	}
}

func TestUpgradeResetDropsTheRenamedValue(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	storeValues(t, cluster, map[string]any{keySecurity: map[string]any{keyOld: map[string]any{keyHours: oldHours}}})
	result := upgrade(t, kubeconfig, current, resetFlag, setFlag, "app.auth.bootstrap.admin="+adminEmail)
	testutil.Equal(t, "names the key", strings.Contains(result.Stdout, keyOld), false)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyWait, data(cluster, keyWait), chartWait)
}

func TestTheChartRefusesTheRenamedValue(t *testing.T) {
	_, kubeconfig := start(t)
	result := install(t, kubeconfig, current, setFlag, "app.security.breakGlass.waitHours=48")
	testutil.Contains(t, stderrLabel, result.Stderr, oldKeyRefuse)
	testutil.Equal(t, codeLabel, result.Code, failed)
}
