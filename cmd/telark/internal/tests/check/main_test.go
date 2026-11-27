package check

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	namespace      = "telark"
	cliVersion     = "0.0.0-dev"
	crdVersion     = "0.2.1"
	partOf         = "telark"
	exporter       = "exporter-service"
	auth           = "auth-service"
	controller     = "admission-controller"
	redis          = "telark-redis-master"
	authPod        = "telark-auth-service-0"
	adminEmail     = "test@example.com"
	kyvernoWebhook = "kyverno-resource-validating-webhook-cfg"
	guardPolicy    = "telark-crd-guard"
	guardBinding   = "telark-crd-guard-binding"
	telarkGroup    = "telark.io/v1alpha1"
	configKind     = "TelarkConfig"
	planKind       = "ProtectionPlan"
	configName     = "default"
	pinnedAt       = "2026-01-01T00:00:00Z"
	envAdmin       = "BOOTSTRAP_ADMIN"
	envWait        = "AUTH_REQUEST_WAIT_HOURS"
	checkCommand   = "check"
	contextName    = "test"
	codeLabel      = "code"
	stdoutLabel    = "stdout"
	stderrLabel    = "stderr"
	keyStatus      = "status"
	keyBootstrap   = "bootstrap"
	keyHealth      = "health"
	healthy        = "healthy"
	nameLabel      = "app.kubernetes.io/name"
	kyvernoName    = "kyverno-admission-controller"
	webhookLabel   = "webhook.kyverno.io/managed-by"
	telarkSuffix   = ".telark.io"
	ready          = 1
	down           = 0
	noRestarts     = 0
)

var github *testutil.GitHub

func TestMain(m *testing.M) {
	github = testutil.InstallGitHub()
	m.Run()
}

func run(t *testing.T, cluster testutil.Cluster, args ...string) testutil.Result {
	t.Helper()
	kubeconfig := testutil.Kubeconfig(t, contextName, map[string]string{contextName: testutil.ClusterServer(t, cluster)})
	return testutil.Run(t, append([]string{checkCommand, "--kubeconfig", kubeconfig}, args...)...)
}

func with(objects []runtime.Object, replacements ...runtime.Object) []runtime.Object {
	for _, replacement := range replacements {
		objects = append(without(objects, key(replacement)), replacement)
	}
	return objects
}

func without(objects []runtime.Object, keys ...string) []runtime.Object {
	return slices.DeleteFunc(slices.Clone(objects), func(object runtime.Object) bool {
		return slices.Contains(keys, key(object)) || slices.Contains(keys, name(object))
	})
}

func key(object runtime.Object) string {
	return fmt.Sprintf("%s/%s/%s", reflect.TypeOf(object), object.GetObjectKind().GroupVersionKind().Kind, name(object))
}

func name(object runtime.Object) string {
	accessor, err := meta.Accessor(object)
	if err != nil {
		panic(err)
	}
	return accessor.GetName()
}
