package version

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"sigs.k8s.io/yaml"
)

const (
	namespace      = "telark"
	partOf         = "telark"
	contextName    = "test"
	serverLabel    = "0.2.3"
	exporter       = "exporter-service"
	versionCommand = "version"
	kubeconfigFlag = "--kubeconfig"
	outputFlag     = "-o"
	jsonFormat     = "json"
	codeLabel      = "code"
	stdoutLabel    = "stdout"
	stderrLabel    = "stderr"
	missingFile    = "/nonexistent/kubeconfig"
	wantServices   = 3
	skewWarning    = "‼ The CLI (0.0.0-dev) and the server (0.2.3) are from different releases: use the CLI that matches the server\n"
)

var clientTable = "Client:\n" +
	"  Version:     0.0.0-dev\n" +
	"  Git commit:  unknown\n" +
	"  Build date:  unknown\n" +
	"  Go version:  " + runtime.Version() + "\n"

// The kyverno Deployment and the one in another namespace must stay out of the report.
func telarkCluster(t *testing.T, label string) string {
	t.Helper()
	server := testutil.APIServer(t,
		testutil.Deployment(namespace, exporter, partOf, label, "ghcr.io/telark/exporter:0.2.3"),
		testutil.Deployment(namespace, "analyzer-service", partOf, label, "ghcr.io/telark/analyzer:0.1.5@sha256:0123"),
		testutil.Deployment(namespace, "auth-service", partOf, label, "localhost:5000/telark/auth"),
		testutil.Deployment(namespace, "admission-controller", "telark-kyverno", "3.9.2", "reg.kyverno.io/kyverno/kyverno:v1.19.2"),
		testutil.Deployment("staging", exporter, partOf, "9.9.9", "ghcr.io/telark/exporter:9.9.9"),
	)
	return testutil.Kubeconfig(t, contextName, map[string]string{contextName: server})
}

func TestClientOnly(t *testing.T) {
	result := testutil.Run(t, versionCommand, "--client", kubeconfigFlag, missingFile)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
	testutil.Equal(t, stdoutLabel, result.Stdout, clientTable)
	testutil.Equal(t, stderrLabel, result.Stderr, "")
}

func TestServerTable(t *testing.T) {
	result := testutil.Run(t, versionCommand, kubeconfigFlag, telarkCluster(t, serverLabel))
	want := clientTable +
		"Server:\n" +
		"  Version:     0.2.3\n" +
		"  Namespace:   telark\n" +
		"  Services:\n" +
		"    analyzer   0.1.5\n" +
		"    auth       localhost:5000/telark/auth\n" +
		"    exporter   0.2.3\n"
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
	testutil.Equal(t, stdoutLabel, result.Stdout, want)
	testutil.Equal(t, stderrLabel, result.Stderr, skewWarning)
}

func TestStructuredOutput(t *testing.T) {
	decoders := map[string]func([]byte, any) error{
		jsonFormat: json.Unmarshal,
		"yaml":     func(data []byte, v any) error { return yaml.Unmarshal(data, v) },
	}
	for format, decode := range decoders {
		t.Run(format, func(t *testing.T) {
			result := testutil.Run(t, versionCommand, outputFlag, format, kubeconfigFlag, telarkCluster(t, serverLabel))
			var got map[string]map[string]any
			if err := decode([]byte(result.Stdout), &got); err != nil {
				t.Fatalf("decode %q: %v", result.Stdout, err)
			}
			testutil.Equal(t, "client version", got["clientVersion"]["version"], any("0.0.0-dev"))
			testutil.Equal(t, "git commit", got["clientVersion"]["gitCommit"], any("unknown"))
			testutil.Equal(t, "build date", got["clientVersion"]["buildDate"], any("unknown"))
			testutil.Equal(t, "go version", got["clientVersion"]["goVersion"], any(runtime.Version()))
			testutil.Equal(t, "server version", got["serverVersion"]["version"], any(serverLabel))
			testutil.Equal(t, "namespace", got["serverVersion"]["namespace"], any(namespace))
			services, ok := got["serverVersion"]["services"].(map[string]any)
			if !ok {
				t.Fatalf("services: got %T", got["serverVersion"]["services"])
			}
			testutil.Equal(t, "services", len(services), wantServices)
			testutil.Equal(t, "analyzer", services["analyzer"], any("0.1.5"))
			testutil.Equal(t, stderrLabel, result.Stderr, skewWarning)
		})
	}
}

func TestClientOnlyJSON(t *testing.T) {
	result := testutil.Run(t, versionCommand, "--client", outputFlag, jsonFormat)
	var got map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", result.Stdout, err)
	}
	_, hasServer := got["serverVersion"]
	testutil.Equal(t, "server part", hasServer, false)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
}

func TestNamespaceFlag(t *testing.T) {
	result := testutil.Run(t, versionCommand, "-n", "staging", outputFlag, jsonFormat, kubeconfigFlag, telarkCluster(t, serverLabel))
	testutil.Contains(t, stdoutLabel, result.Stdout, `"version": "9.9.9"`)
	testutil.Contains(t, stdoutLabel, result.Stdout, `"namespace": "staging"`)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
}

// Only a major or minor gap between the CLI (0.0.0-dev in tests) and the server warns.
func TestVersionSkew(t *testing.T) {
	cases := []struct {
		label    string
		wantWarn bool
	}{
		{"0.0.9", false},
		{"0.1.0", true},
		{"1.0.0", true},
		{"not-a-version", false},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			result := testutil.Run(t, versionCommand, kubeconfigFlag, telarkCluster(t, c.label))
			testutil.Equal(t, "warned", result.Stderr != "", c.wantWarn)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
		})
	}
}

// A half-finished upgrade leaves Deployments on two releases: the report shows both, and a gap to either one warns.
func TestMixedVersions(t *testing.T) {
	cases := []struct {
		name        string
		newer       string
		wantVersion string
		wantWarn    string
	}{
		{"patch gap only", "0.0.9", `"version": "0.0.1, 0.0.9"`, ""},
		{"minor gap on one", "0.1.0", `"version": "0.0.1, 0.1.0"`, "‼ The CLI (0.0.0-dev) and the server (0.0.1, 0.1.0) are from different releases"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := testutil.APIServer(t,
				testutil.Deployment(namespace, exporter, partOf, c.newer, "ghcr.io/telark/exporter:"+c.newer),
				testutil.Deployment(namespace, "auth-service", partOf, "0.0.1", "ghcr.io/telark/auth:0.0.1"),
			)
			kubeconfig := testutil.Kubeconfig(t, contextName, map[string]string{contextName: server})
			result := testutil.Run(t, versionCommand, outputFlag, jsonFormat, kubeconfigFlag, kubeconfig)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
			testutil.Contains(t, stdoutLabel, result.Stdout, c.wantVersion)
			testutil.Contains(t, stderrLabel, result.Stderr, c.wantWarn)
			testutil.Equal(t, "warned", result.Stderr != "", c.wantWarn != "")
		})
	}
}
