package kube

import (
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
)

const (
	namespace   = "telark"
	partOf      = "telark"
	current     = "current"
	other       = "other"
	contextFlag = "--context"
)

// Two clusters that report different Telark versions, so each test can tell which one the CLI reached.
func twoClusters(t *testing.T) string {
	t.Helper()
	servers := map[string]string{
		current: testutil.APIServer(t, testutil.Deployment(namespace, "exporter-service", partOf, "0.2.3", "ghcr.io/telark/exporter:0.2.3")),
		other:   testutil.APIServer(t, testutil.Deployment(namespace, "exporter-service", partOf, "0.3.0", "ghcr.io/telark/exporter:0.3.0")),
	}
	return testutil.Kubeconfig(t, current, servers)
}

func TestClusterSelection(t *testing.T) {
	cases := []struct {
		name        string
		env         bool
		args        []string
		wantVersion string
	}{
		{"kubeconfig flag, current context", false, nil, `"version": "0.2.3"`},
		{"kubeconfig flag, context flag", false, []string{contextFlag, other}, `"version": "0.3.0"`},
		{"KUBECONFIG variable", true, nil, `"version": "0.2.3"`},
		{"KUBECONFIG variable, context flag", true, []string{contextFlag, other}, `"version": "0.3.0"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kubeconfig := twoClusters(t)
			args := []string{"version", "-o", "json"}
			if c.env {
				t.Setenv("KUBECONFIG", kubeconfig)
			} else {
				args = append(args, "--kubeconfig", kubeconfig)
			}
			result := testutil.Run(t, append(args, c.args...)...)
			testutil.Equal(t, "code", result.Code, constants.ExitCodeSuccess)
			testutil.Contains(t, "stdout", result.Stdout, c.wantVersion)
		})
	}
}

func TestCompleteContexts(t *testing.T) {
	result := testutil.Run(t, "__complete", "--kubeconfig", twoClusters(t), contextFlag, "")
	testutil.Equal(t, "completions", result.Stdout, "current\nother\n:4\n")
}

// With no kubeconfig file, or one without a current context, the error names the flags that pick a cluster.
func TestNoClusterSelected(t *testing.T) {
	cases := map[string]string{
		"KUBECONFIG names a missing file": "/nonexistent/kubeconfig",
		"no current context":              testutil.Kubeconfig(t, "", map[string]string{current: testutil.ClosedServer(t)}),
	}
	for name, kubeconfig := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("KUBECONFIG", kubeconfig)
			result := testutil.Run(t, "version")
			testutil.Equal(t, "code", result.Code, constants.ExitCodeFailure)
			testutil.Contains(t, "stderr", result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
		})
	}
}

func TestCompleteContextsWithoutKubeconfig(t *testing.T) {
	result := testutil.Run(t, "__complete", "--kubeconfig", "/nonexistent/kubeconfig", contextFlag, "")
	testutil.Equal(t, "completions", result.Stdout, ":1\n")
}
