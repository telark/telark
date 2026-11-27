package install

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	namespace           = "telark"
	adminEmail          = "test@example.com"
	contextName         = "test"
	installCommand      = "install"
	upgradeCommand      = "upgrade"
	kubeconfigFlag      = "--kubeconfig"
	adminFlag           = "--bootstrap-admin"
	versionFlag         = "--version"
	directoryFlag       = "--chart-directory"
	dryRunFlag          = "--dry-run"
	waitFlag            = "--wait"
	setFlag             = "--set"
	chartDirectory      = "testdata/telark"
	oldest              = "0.1.0"
	previous            = "0.2.0"
	current             = "0.2.1"
	candidate           = "0.3.0-rc.1"
	next                = "0.3.0"
	broken              = "0.3.1-broken"
	settings            = "telark-settings"
	configMaps          = "configmaps"
	secrets             = "secrets"
	firstRelease        = "sh.helm.release.v1.telark.v1"
	secondRelease       = "sh.helm.release.v1.telark.v2"
	cosignName          = "cosign"
	pathVariable        = "PATH"
	codeLabel           = "code"
	stdoutLabel         = "stdout"
	stderrLabel         = "stderr"
	writesLabel         = "writes"
	releaseLabel        = "release status"
	secondLabel         = "second release"
	keyMode             = "mode"
	keyChart            = "chart"
	deployed            = "deployed"
	extraEnabled        = "extra.enabled=true"
	none                = constants.DefaultInitValue
	executable          = 0o700
	readable            = 0o600
	registryConfigError = "Error: failed to decode config file "
	unlimited           = 1000
	failed              = constants.ExitCodeFailure
	passed              = constants.ExitCodeSuccess
	noCosign            = "‼ cosign isn't installed, so the chart's signature wasn't checked: install cosign to check it\n"
	unpublished         = "Error: pull the chart ghcr.io/telark/charts/telark:0.0.0-dev: "
	writeFailed         = "Error: write failed\n"
)

var registry *testutil.Registry

// Helm, the registry client and client-go read and cache under the home directory, and the chart is verified with
// whatever cosign PATH holds: the tests get an empty home and an empty PATH instead of the developer's.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "telark-install-tests")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(home) }()
	for _, name := range []string{"HELM_CONFIG_HOME", "HELM_CACHE_HOME", "HELM_DATA_HOME", "DOCKER_CONFIG", "KUBECACHEDIR", pathVariable} {
		if err := os.Setenv(name, home); err != nil {
			panic(err)
		}
	}
	registry = testutil.InstallRegistry()
	registry.Publish(chartDirectory, oldest, previous, current, candidate, next)
	registry.PublishBroken(broken)
	m.Run()
}

func start(t *testing.T, objects ...runtime.Object) (cluster *testutil.FakeCluster, kubeconfig string) {
	t.Helper()
	cluster = testutil.StartCluster(t, testutil.Cluster{Objects: append(testutil.FreshCluster(), objects...)})
	return cluster, testutil.Kubeconfig(t, contextName, map[string]string{contextName: cluster.URL})
}

func install(t *testing.T, kubeconfig, version string, args ...string) testutil.Result {
	t.Helper()
	return testutil.Run(t, append([]string{installCommand, kubeconfigFlag, kubeconfig, adminFlag, adminEmail, versionFlag, version}, args...)...)
}

func upgrade(t *testing.T, kubeconfig, version string, args ...string) testutil.Result {
	t.Helper()
	return testutil.Run(t, append([]string{upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, version}, args...)...)
}

// A release is installed through the CLI itself, and the services a chart would start are added once it is in.
func released(t *testing.T, version string, args ...string) (cluster *testutil.FakeCluster, kubeconfig string) {
	t.Helper()
	cluster, kubeconfig = start(t)
	if result := install(t, kubeconfig, version, args...); result.Code != passed {
		t.Fatalf("install %s: %s", version, result.Stderr)
	}
	cluster.Add(t, testutil.HealthyTelark()...)
	return cluster, kubeconfig
}

// The script records its arguments beside itself, then runs body.
func cosign(t *testing.T, body string) (arguments func() string) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\necho \"$@\" > \"$0.args\"\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(directory, cosignName), []byte(script), executable); err != nil {
		t.Fatal(err)
	}
	t.Setenv(pathVariable, directory)
	return func() string {
		sent, err := fs.ReadFile(os.DirFS(directory), cosignName+".args")
		if err != nil {
			t.Fatal(err)
		}
		return string(sent)
	}
}

func data(cluster *testutil.FakeCluster, key string) string {
	value, _, _ := unstructured.NestedString(cluster.Find(configMaps, settings), "data", key)
	return value
}

func status(cluster *testutil.FakeCluster, release string) string {
	value, _, _ := unstructured.NestedString(cluster.Find(secrets, release), "metadata", "labels", "status")
	return value
}

func unreadableRegistryConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "registry"), executable); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "registry", "config.json"), []byte("{"), readable); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HELM_CONFIG_HOME", home)
}

func category(name string, spec map[string]any) runtime.Object {
	return testutil.Resource("telark.io/v1alpha1", "Category", namespace, name, map[string]any{"spec": spec})
}
