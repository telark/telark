package version

import (
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// A server error still prints the client part first, then fails with exit code 1.
func TestServerErrors(t *testing.T) {
	cases := []struct {
		name       string
		kubeconfig func(t *testing.T) string
		args       []string
		wantError  string
	}{
		{"no deployments", func(t *testing.T) string { return telarkCluster(t, serverLabel) }, []string{"-n", "empty"},
			`Error: no Telark deployments in namespace "empty": pass the namespace Telark runs in with -n`},
		{"forbidden", forbiddenCluster, nil, "deployments.apps is forbidden"},
		{"unreachable", closedCluster, nil, "list the Telark deployments in namespace \"telark\""},
		{"missing context", func(t *testing.T) string { return telarkCluster(t, serverLabel) }, []string{"--context", "gone"}, "gone"},
		{"bad certificate authority", badCACluster, nil, "root certificates"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{versionCommand, kubeconfigFlag, c.kubeconfig(t)}, c.args...)
			result := testutil.Run(t, args...)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Equal(t, stdoutLabel, result.Stdout, clientTable)
			testutil.Contains(t, stderrLabel, result.Stderr, c.wantError)
		})
	}
}

func TestMissingKubeconfig(t *testing.T) {
	result := testutil.Run(t, versionCommand, kubeconfigFlag, missingFile)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
	testutil.Contains(t, stderrLabel, result.Stderr, missingFile)
}

func TestWriteErrors(t *testing.T) {
	for _, format := range []string{"table", jsonFormat} {
		t.Run(format, func(t *testing.T) {
			result := testutil.RunTo(t, testutil.FailingWriter{}, versionCommand, "--client", outputFlag, format)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Equal(t, stderrLabel, result.Stderr, "Error: "+testutil.ErrWrite.Error()+"\n")
		})
	}
}

func TestExtraArguments(t *testing.T) {
	result := testutil.Run(t, versionCommand, "extra")
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
	testutil.Contains(t, stderrLabel, result.Stderr, `unknown command "extra" for "telark version"`)
}

func forbiddenCluster(t *testing.T) string {
	t.Helper()
	return testutil.Kubeconfig(t, contextName, map[string]string{contextName: testutil.ForbiddenAPIServer(t)})
}

func closedCluster(t *testing.T) string {
	t.Helper()
	return testutil.Kubeconfig(t, contextName, map[string]string{contextName: testutil.ClosedServer(t)})
}

func badCACluster(t *testing.T) string {
	t.Helper()
	config := clientcmdapi.NewConfig()
	config.Clusters[contextName] = &clientcmdapi.Cluster{Server: "https://127.0.0.1:1", CertificateAuthorityData: []byte("not a certificate")}
	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{}
	config.Contexts[contextName] = &clientcmdapi.Context{Cluster: contextName, AuthInfo: contextName}
	config.CurrentContext = contextName
	return testutil.WriteKubeconfig(t, config)
}
