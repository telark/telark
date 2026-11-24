package install

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
)

const (
	modeFlag  = "--mode"
	preChecks = `✔ Kubernetes v1.34.0 is 1.30 or newer
✔ You can create the 20 kinds of objects the chart installs
✔ standard is the default StorageClass, for the volume claims
✔ No other Kyverno runs in the cluster
✔ No data from an earlier install
✔ The nodes have 4.00 CPU and 8.0 GiB free, enough for the standard mode's 1.65 CPU and 3.3 GiB
`
	installedPrevious = `Installed Telark 0.2.0 in namespace "telark".
Next:
  telark check
  telark auth enroll --email test@example.com
`
	dryRunOutput = `Dry run of Telark 0.2.0 in namespace "telark": nothing changed.
Values:
app:
  auth:
    bootstrap:
      admin: test@example.com
Objects:
  ConfigMap                          1
  CustomResourceDefinition           2
`
	installWrites = "PATCH configmaps/telark-settings, PATCH customresourcedefinitions.apiextensions.k8s.io/accessroles.telark.io, " +
		"PATCH customresourcedefinitions.apiextensions.k8s.io/categories.telark.io, PATCH namespaces/telark, POST secrets/, " +
		"PUT secrets/sh.helm.release.v1.telark.v1"
	cosignArguments = "verify ghcr.io/telark/charts/telark@%s " +
		"--certificate-identity https://github.com/telark/telark/.github/workflows/release-charts.yaml@refs/tags/v0.2.0 " +
		"--certificate-oidc-issuer https://token.actions.githubusercontent.com\n"
	unbuiltDirectory = "testdata/unbuilt"
	valuesFile       = "app:\n  mode: minimal\n  persistence:\n    storageClass: standard\n"
	valuesName       = "values.yaml"
)

func TestInstall(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := install(t, kubeconfig, previous)
	testutil.Equal(t, stdoutLabel, result.Stdout, preChecks+noCosign+installedPrevious)
	testutil.Equal(t, stderrLabel, result.Stderr, "")
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyMode, data(cluster, keyMode), "standard")
	testutil.Equal(t, "admin", data(cluster, "admin"), adminEmail)
	testutil.Equal(t, keyChart, data(cluster, keyChart), previous)
	testutil.Equal(t, releaseLabel, status(cluster, firstRelease), deployed)
	testutil.Equal(t, "namespace created", cluster.Find("namespaces", namespace) != nil, true)
	writes := cluster.Writes()
	requests := make([]string, none, len(writes))
	for _, write := range writes {
		requests = append(requests, write.Request)
		if strings.HasPrefix(write.Request, "PATCH") {
			testutil.Equal(t, write.Request+" field manager", write.Query.Get("fieldManager"), "helm")
		}
	}
	slices.Sort(requests)
	testutil.Equal(t, writesLabel, strings.Join(requests, ", "), installWrites)
}

func TestInstallValues(t *testing.T) {
	file := filepath.Join(t.TempDir(), valuesName)
	if err := os.WriteFile(file, []byte(valuesFile), readable); err != nil {
		t.Fatal(err)
	}
	cluster, kubeconfig := start(t, testutil.KyvernoWebhook("kyverno-resource-validating-webhook-cfg"))
	result := testutil.Run(t, installCommand, kubeconfigFlag, kubeconfig, versionFlag, previous, "-f", file,
		setFlag, "kyverno.enabled=false", setFlag, "app.auth.bootstrap.admin="+adminEmail)
	for _, line := range []string{
		"✔ standard exists, for the volume claims\n",
		"✔ Kyverno runs in the cluster, and Telark will use it (kyverno.enabled=false)\n",
		"enough for the minimal mode's 1.35 CPU and 2.9 GiB\n",
		"telark auth enroll --email test@example.com\n",
	} {
		testutil.Contains(t, stdoutLabel, result.Stdout, line)
	}
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyMode, data(cluster, keyMode), "minimal")
}

func TestInstallChecksWhatTheValuesSet(t *testing.T) {
	cases := map[string]checkCase{
		"volumes the operator creates": {
			args: []string{setFlag, "app.persistence.storageClass=-"},
			line: "✔ app.persistence.storageClass=- binds the volume claims to volumes you create\n",
		},
		"a StorageClass the cluster lacks": {
			args: []string{setFlag, "app.persistence.storageClass=fast"}, code: failed,
			line: "× app.persistence.storageClass=\"fast\" doesn't exist in this cluster (kubectl get storageclass lists them)\n",
		},
		"a Kyverno to use that doesn't run": {
			args: []string{setFlag, "kyverno.enabled=false"}, code: failed,
			line: "× kyverno.enabled=false needs a Kyverno that already runs, and none has its webhooks registered\n",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, kubeconfig := start(t)
			result := install(t, kubeconfig, previous, c.args...)
			testutil.Contains(t, stdoutLabel, result.Stdout, c.line)
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func TestInstallModeFlagWins(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := install(t, kubeconfig, previous, setFlag, "app.mode=minimal", modeFlag, "performance")
	testutil.Contains(t, stdoutLabel, result.Stdout, "enough for the performance mode's 2.55 CPU and 5.6 GiB\n")
	testutil.Equal(t, keyMode, data(cluster, keyMode), "performance")
}

func TestInstallRefusesBadSettings(t *testing.T) {
	cases := map[string]refusalCase{
		"no bootstrap admin": {nil, "Error: pass --bootstrap-admin <email>: the chart requires the bootstrap admin (app.auth.bootstrap.admin)\n"},
		"an unknown mode": {[]string{adminFlag, adminEmail, modeFlag, "tiny"},
			"Error: app.mode \"tiny\": must be minimal, standard or performance\n"},
		"values without an app section": {[]string{setFlag, "app=telark"},
			"Error: pass --bootstrap-admin <email>: the chart requires the bootstrap admin (app.auth.bootstrap.admin)\n"},
		"a missing values file": {[]string{adminFlag, adminEmail, "-f", "testdata/missing.yaml"},
			"Error: open testdata/missing.yaml: no such file or directory\n"},
		"an argument": {[]string{"extra"}, "Error: unknown command \"extra\" for \"telark install\"\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := start(t)
			result := testutil.Run(t, append([]string{installCommand, kubeconfigFlag, kubeconfig}, c.args...)...)
			testutil.Equal(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, stdoutLabel, result.Stdout, "")
			testutil.Equal(t, codeLabel, result.Code, failed)
			testutil.Equal(t, writesLabel, len(cluster.Writes()), none)
		})
	}
}

func TestInstallWithoutACluster(t *testing.T) {
	t.Setenv("KUBECONFIG", t.TempDir()+"/missing")
	result := testutil.Run(t, installCommand, adminFlag, adminEmail)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestInstallStopsOnAFailedPreCheck(t *testing.T) {
	cluster, kubeconfig := start(t, testutil.KyvernoWebhook("kyverno-resource-validating-webhook-cfg"))
	result := install(t, kubeconfig, previous)
	testutil.Contains(t, stdoutLabel, result.Stdout, "× Kyverno already runs in this cluster (kyverno-resource-validating-webhook-cfg)")
	testutil.Equal(t, "signature checked", strings.Contains(result.Stdout, "signature"), false)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: 1 of 6 checks failed\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
	testutil.Equal(t, writesLabel, len(cluster.Writes()), none)
}

func TestInstallChartErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	cases := map[string]refusalCase{
		"the CLI's own version, never published": {nil, unpublished},
		"an archive that isn't a chart": {
			[]string{versionFlag, broken}, "Error: load the chart from ghcr.io/telark/charts/telark:0.3.1-broken: ",
		},
		"a missing chart directory": {[]string{directoryFlag, missing}, "Error: load the chart from " + missing + ": "},
		"a chart directory without its subcharts": {[]string{directoryFlag, unbuiltDirectory},
			"Error: the chart in testdata/unbuilt lacks its subcharts, so build them first (make deps, or helm dependency build): "},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := start(t)
			result := testutil.Run(t, append([]string{installCommand, kubeconfigFlag, kubeconfig, adminFlag, adminEmail}, c.args...)...)
			testutil.Equal(t, stdoutLabel, result.Stdout, preChecks)
			testutil.Contains(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, failed)
			testutil.Equal(t, writesLabel, len(cluster.Writes()), none)
		})
	}
}

func TestInstallWithAnUnreadableRegistryConfig(t *testing.T) {
	unreadableRegistryConfig(t)
	_, kubeconfig := start(t)
	result := install(t, kubeconfig, previous)
	testutil.Contains(t, stderrLabel, result.Stderr, registryConfigError)
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestInstallSignature(t *testing.T) {
	cases := map[string]signatureCase{
		"a valid signature": {
			script: "exit 0", line: "✔ The chart's signature is valid: ghcr.io/telark/charts/telark@" + registry.Digest(previous) + "\n",
		},
		"a signature that doesn't verify": {
			script: "echo 'Error: no matching signatures' >&2\necho 'error during command execution: no signatures found' >&2\nexit 1",
			line:   "× The chart's signature doesn't verify: error during command execution: no signatures found\n",
			stderr: "Error: 1 of 1 checks failed\n", code: failed,
		},
		"a cosign that fails without a word": {
			script: "exit 3", line: "× The chart's signature doesn't verify: exit status 3\n",
			stderr: "Error: 1 of 1 checks failed\n", code: failed,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			arguments := cosign(t, c.script)
			cluster, kubeconfig := start(t)
			result := install(t, kubeconfig, previous)
			testutil.Contains(t, stdoutLabel, result.Stdout, preChecks+c.line)
			testutil.Equal(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, c.code)
			testutil.Equal(t, "installed", cluster.Find(secrets, firstRelease) != nil, c.code == passed)
			testutil.Equal(t, "cosign arguments", arguments(), fmt.Sprintf(cosignArguments, registry.Digest(previous)))
		})
	}
}

func TestInstallFromAChartDirectory(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := testutil.Run(t, installCommand, kubeconfigFlag, kubeconfig, adminFlag, adminEmail, directoryFlag, chartDirectory)
	testutil.Contains(t, stdoutLabel, result.Stdout, "‼ The chart comes from testdata/telark, so its signature isn't checked\n"+
		`Installed Telark 0.0.0 in namespace "telark".`)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, keyChart, data(cluster, keyChart), "0.0.0")
}

func TestInstallOverARelease(t *testing.T) {
	_, kubeconfig := start(t)
	install(t, kubeconfig, previous)
	result := install(t, kubeconfig, previous)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: install: release name check failed: cannot reuse a name that is still in use\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestInstallWaits(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := install(t, kubeconfig, previous, waitFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, preChecks+noCosign+installedPrevious)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, releaseLabel, status(cluster, firstRelease), deployed)
}

func TestInstallDryRun(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := install(t, kubeconfig, previous, dryRunFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, preChecks+noCosign+dryRunOutput)
	testutil.Equal(t, stderrLabel, result.Stderr, "")
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, writesLabel, len(cluster.Writes()), none)
}

func TestInstallDryRunWriteFailures(t *testing.T) {
	_, kubeconfig := start(t)
	arguments := []string{installCommand, kubeconfigFlag, kubeconfig, adminFlag, adminEmail, versionFlag, previous, dryRunFlag}
	counter := &limitedWriter{left: unlimited}
	testutil.Equal(t, codeLabel, testutil.RunTo(t, counter, arguments...).Code, passed)
	for written := range unlimited - counter.left {
		result := testutil.RunTo(t, &limitedWriter{left: written}, arguments...)
		testutil.Equal(t, stderrLabel, result.Stderr, writeFailed)
		testutil.Equal(t, codeLabel, result.Code, failed)
	}
}

func TestInstallWriteFailure(t *testing.T) {
	cluster, kubeconfig := start(t)
	result := testutil.RunTo(t, testutil.FailingWriter{}, installCommand, kubeconfigFlag, kubeconfig, adminFlag, adminEmail, versionFlag, previous)
	testutil.Equal(t, stderrLabel, result.Stderr, writeFailed)
	testutil.Equal(t, codeLabel, result.Code, failed)
	testutil.Equal(t, releaseLabel, status(cluster, firstRelease), deployed)
}

func TestInstallCompletesModes(t *testing.T) {
	result := testutil.Run(t, "__complete", installCommand, modeFlag, "")
	testutil.Equal(t, "completions", result.Stdout, "minimal\nstandard\nperformance\n:4\n")
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.left == none {
		return none, testutil.ErrWrite
	}
	w.left--
	return len(p), nil
}
