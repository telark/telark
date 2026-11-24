package install

import (
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	resetFlag     = "--reset-values"
	upgradeChecks = `✔ 0.2.0 → 0.2.1 is the next release
✔ The server accepts every object of the upgrade (3, dry run)
✔ Every stored object (0) fits the 0.2.1 schemas
✔ No recovery request is open
✔ No protection plan enforces now
`
	upgradedOutput = `Upgraded Telark to 0.2.1 in namespace "telark".
✔ 10 CRDs are established (telark-crds 0.2.1)
✔ Every Deployment (3) and StatefulSet (1) is available
✔ No pod restarted in the last hour
✔ The CLI (0.0.0-dev) and the server (0.0.0-dev) are from the same release
✔ Kyverno's admission controller is available and its webhooks are registered
✔ The CRD guard is enforced
✔ The bootstrap account test@example.com is pinned
✔ No recovery request is open
✔ No protection plan is drifted or degraded
`
	upgradeDryRun  = "Dry run of the upgrade to 0.2.1: nothing changed.\n"
	waitingMessage = "Waiting up to 5m0s for every check to pass\n"
	crdRequest     = "PATCH customresourcedefinitions.apiextensions.k8s.io/categories.telark.io"
	rolesRequest   = "PATCH customresourcedefinitions.apiextensions.k8s.io/accessroles.telark.io"
	settingsPatch  = "PATCH configmaps/telark-settings"
	extraName      = "telark-extra"
)

func TestUpgrade(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	result := upgrade(t, kubeconfig, current)
	testutil.Equal(t, stdoutLabel, result.Stdout, noCosign+upgradeChecks+upgradedOutput)
	testutil.Equal(t, stderrLabel, result.Stderr, waitingMessage)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyChart, data(cluster, keyChart), current)
	testutil.Equal(t, "first release", status(cluster, firstRelease), "superseded")
	testutil.Equal(t, secondLabel, status(cluster, secondRelease), deployed)
}

func TestUpgradeProbesEveryObjectOnTheServer(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	upgrade(t, kubeconfig, current, dryRunFlag)
	sent := probes(cluster)
	probed := make([]string, none, len(sent))
	for _, write := range sent {
		probed = append(probed, write.Request)
		testutil.Equal(t, "query", write.Query.Encode(), "dryRun=All&fieldManager=helm&fieldValidation=Strict&force=false")
		testutil.Equal(t, "managed by", stringMap(write.Object, "labels")["app.kubernetes.io/managed-by"], "Helm")
		testutil.Equal(t, "release name", stringMap(write.Object, "annotations")["meta.helm.sh/release-name"], "telark")
		testutil.Equal(t, "release namespace", stringMap(write.Object, "annotations")["meta.helm.sh/release-namespace"], namespace)
	}
	testutil.Equal(t, "probes", strings.Join(probed, ", "), settingsPatch+", "+crdRequest+", "+rolesRequest)
	testutil.Equal(t, "chart label kept", stringMap(sent[1].Object, "labels")["app.kubernetes.io/version"], current)
}

func TestUpgradeForcesTheProbeOfAClientSideRelease(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
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
	release.ApplyMethod = string(releasev1.ApplyMethodClientSideApply)
	if err := releases.Update(firstRelease, release); err != nil {
		t.Fatal(err)
	}
	result := upgrade(t, kubeconfig, current, dryRunFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, noCosign+upgradeChecks+upgradeDryRun)
	for _, write := range probes(cluster) {
		testutil.Equal(t, write.Request+" force", write.Query.Get("force"), "true")
	}
}

func TestUpgradeKeepsTheValuesUnlessReset(t *testing.T) {
	cluster, kubeconfig := released(t, previous, modeFlag, "minimal")
	testutil.Equal(t, codeLabel, upgrade(t, kubeconfig, current).Code, passed)
	testutil.Equal(t, "kept mode", data(cluster, keyMode), "minimal")
	testutil.Equal(t, "kept admin", data(cluster, "admin"), adminEmail)
	result := upgrade(t, kubeconfig, current, resetFlag, setFlag, "app.auth.bootstrap.admin="+adminEmail)
	testutil.Contains(t, stdoutLabel, result.Stdout, "✔ 0.2.1 is the release already installed\n")
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, "reset mode", data(cluster, keyMode), "standard")
}

func TestUpgradeRemovesWhatTheChartDrops(t *testing.T) {
	cluster, kubeconfig := released(t, previous, setFlag, extraEnabled)
	testutil.Equal(t, "installed", cluster.Find(configMaps, extraName) != nil, true)
	result := upgrade(t, kubeconfig, current, setFlag, "extra.enabled=false")
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, "removed", cluster.Find(configMaps, extraName) == nil, true)
}

func TestUpgradeWaits(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	result := upgrade(t, kubeconfig, current, waitFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, noCosign+upgradeChecks+upgradedOutput)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, secondLabel, status(cluster, secondRelease), deployed)
}

func TestUpgradeDryRun(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	result := upgrade(t, kubeconfig, current, dryRunFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, noCosign+upgradeChecks+upgradeDryRun)
	testutil.Equal(t, stderrLabel, result.Stderr, "")
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyChart, data(cluster, keyChart), previous)
	testutil.Equal(t, "first release", status(cluster, firstRelease), deployed)
	testutil.Equal(t, secondLabel, cluster.Find(secrets, secondRelease) == nil, true)
}

func TestUpgradeStep(t *testing.T) {
	cases := map[string]stepCase{
		"the installed release": {installed: current, target: current, line: "✔ 0.2.1 is the release already installed\n"},
		"an older release": {
			installed: current, target: previous, code: failed,
			line: "× 0.2.0 is older than the installed 0.2.1: go back with helm rollback instead\n",
		},
		"the next release, past its candidate": {installed: current, target: next, line: "✔ 0.2.1 → 0.3.0 is the next release\n"},
		"skipped releases": {
			installed: oldest, target: next, code: failed,
			line: "× 0.1.0 → 0.3.0 skips 0.2.0, 0.2.1: upgrade one release at a time\n",
		},
		"an unlisted registry": {
			installed: oldest, target: next, unlisted: true,
			line: "‼ Couldn't list the published releases, so skipped ones aren't checked: ",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, kubeconfig := released(t, c.installed)
			if c.unlisted {
				registry.FailTags(t)
			}
			result := upgrade(t, kubeconfig, c.target, dryRunFlag)
			testutil.Contains(t, stdoutLabel, result.Stdout, noCosign+c.line)
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func TestUpgradeRefusals(t *testing.T) {
	cases := map[string]refusalCase{
		"a missing values file": {
			[]string{"-f", "testdata/missing.yaml"}, "Error: open testdata/missing.yaml: no such file or directory\n",
		},
		"the CLI's own version, never published": {nil, unpublished},
		"an argument":                            {[]string{"extra"}, `Error: unknown command "extra" for "telark upgrade"` + "\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, kubeconfig := released(t, previous)
			result := testutil.Run(t, append([]string{upgradeCommand, kubeconfigFlag, kubeconfig}, c.args...)...)
			testutil.Contains(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, stdoutLabel, result.Stdout, "")
			testutil.Equal(t, codeLabel, result.Code, failed)
		})
	}
}

func TestUpgradeWithoutACluster(t *testing.T) {
	t.Setenv("KUBECONFIG", t.TempDir()+"/missing")
	result := testutil.Run(t, upgradeCommand)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestUpgradeWithoutARelease(t *testing.T) {
	_, kubeconfig := start(t)
	result := upgrade(t, kubeconfig, current)
	testutil.Equal(t, stderrLabel, result.Stderr, `Error: no Telark release in namespace "telark": install it with telark install`+"\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestUpgradeWithAnUnreadableRelease(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	cluster.Refuse("GET secrets/", http.StatusInternalServerError)
	result := upgrade(t, kubeconfig, current)
	testutil.Contains(t, stderrLabel, result.Stderr, "GET secrets/ is refused for this test")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestUpgradeWithAnUnreadableRegistryConfig(t *testing.T) {
	_, kubeconfig := released(t, previous)
	unreadableRegistryConfig(t)
	result := upgrade(t, kubeconfig, current)
	testutil.Contains(t, stderrLabel, result.Stderr, registryConfigError)
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestUpgradeFailure(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	cluster.Refuse("POST secrets/", http.StatusInternalServerError)
	result := upgrade(t, kubeconfig, current)
	testutil.Contains(t, stdoutLabel, result.Stdout, upgradeChecks)
	testutil.Contains(t, stderrLabel, result.Stderr, "Error: upgrade: ")
	testutil.Contains(t, stderrLabel, result.Stderr, "POST secrets/ is refused for this test")
	testutil.Equal(t, codeLabel, result.Code, failed)
	testutil.Equal(t, keyChart, data(cluster, keyChart), previous)
}

func TestUpgradeWriteFailures(t *testing.T) {
	cluster, kubeconfig := released(t, previous)
	result := testutil.RunTo(t, testutil.FailingWriter{}, upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, current, dryRunFlag)
	testutil.Equal(t, stderrLabel, result.Stderr, writeFailed)
	testutil.Equal(t, codeLabel, result.Code, failed)
	result = testutil.RunTo(t, testutil.FailingWriter{}, upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, current)
	testutil.Equal(t, stderrLabel, result.Stderr, writeFailed)
	testutil.Equal(t, codeLabel, result.Code, failed)
	testutil.Equal(t, secondLabel, status(cluster, secondRelease), deployed)
}

func TestUpgradeServerDryRun(t *testing.T) {
	unmanaged := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: extraName, Namespace: namespace}}
	cases := map[string]checkCase{
		"an object another owner holds": {
			objects: []runtime.Object{unmanaged}, args: []string{setFlag, extraEnabled}, code: failed,
			line: "× The server refuses the upgrade (dry run): unable to continue with update: ConfigMap \"telark-extra\" in namespace " +
				"\"telark\" exists and cannot be imported into the current release: invalid ownership metadata",
		},
		"objects the server refuses": {
			refused: map[string]int32{settingsPatch: http.StatusConflict, crdRequest: http.StatusUnprocessableEntity}, code: failed,
			line: "× The server refuses 2 of 3 objects (dry run): configmaps/telark-settings: " + settingsPatch + " is refused for this test, " +
				"customresourcedefinitions/categories.telark.io: " + crdRequest + " is refused for this test\n",
		},
		"one more object": {
			args: []string{setFlag, extraEnabled}, line: "✔ The server accepts every object of the upgrade (4, dry run)\n",
		},
	}
	expect(t, cases)
}

func expect(t *testing.T, cases map[string]checkCase) {
	t.Helper()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, previous)
			cluster.Add(t, c.objects...)
			for request, status := range c.refused {
				cluster.Refuse(request, status)
			}
			result := upgrade(t, kubeconfig, current, append([]string{dryRunFlag}, c.args...)...)
			testutil.Contains(t, stdoutLabel, result.Stdout, c.line)
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func probes(cluster *testutil.FakeCluster) []testutil.Write {
	var probes []testutil.Write
	for _, write := range cluster.Writes() {
		if write.Query.Has("dryRun") {
			probes = append(probes, write)
		}
	}
	return probes
}

func stringMap(object map[string]any, key string) map[string]string {
	values, _, _ := unstructured.NestedStringMap(object, "metadata", key)
	return values
}
