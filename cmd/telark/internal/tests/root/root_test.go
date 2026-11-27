package root

import (
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
)

const (
	codeLabel      = "code"
	helpLabel      = "help"
	versionCommand = "version"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}} {
		result := testutil.Run(t, args...)
		testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
		testutil.Contains(t, helpLabel, result.Stdout, "works with a Telark installation through the Kubernetes API server")
		testutil.Contains(t, helpLabel, result.Stdout, "completion")
		testutil.Contains(t, helpLabel, result.Stdout, versionCommand)
		testutil.Contains(t, helpLabel, result.Stdout, constants.CheckShort)
		testutil.Contains(t, helpLabel, result.Stdout, `-n, --namespace string    The namespace Telark runs in (default "telark")`)
	}
}

func TestVersionHelp(t *testing.T) {
	result := testutil.Run(t, versionCommand, "--help")
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
	testutil.Contains(t, helpLabel, result.Stdout, `-o, --output format   Output format: table, json or yaml (default table)`)
	testutil.Contains(t, helpLabel, result.Stdout, "--kubeconfig string")
	testutil.Contains(t, helpLabel, result.Stdout, "--context string")
}

// Every failure, a usage error included, prints one Error line and exits 1.
func TestErrors(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"bogus"}, `Error: unknown command "bogus" for "telark"`},
		{[]string{"--bogus"}, "Error: unknown flag: --bogus"},
		{[]string{versionCommand, "-o", "xml"}, `Error: invalid argument "xml" for "-o, --output" flag: must be table, json or yaml`},
	}
	for _, c := range cases {
		t.Run(c.args[0], func(t *testing.T) {
			result := testutil.Run(t, c.args...)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Equal(t, "stdout", result.Stdout, "")
			testutil.Contains(t, "stderr", result.Stderr, c.want)
		})
	}
}

func TestCompletionScripts(t *testing.T) {
	cases := map[string]string{
		"bash":       "# bash completion V2 for telark",
		"zsh":        "#compdef telark",
		"fish":       "# fish completion for telark",
		"powershell": "# powershell completion for telark",
	}
	for shell, header := range cases {
		t.Run(shell, func(t *testing.T) {
			result := testutil.Run(t, "completion", shell)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
			testutil.Contains(t, "script", result.Stdout, header)
		})
	}
}

func TestCompleteOutputFormats(t *testing.T) {
	result := testutil.Run(t, "__complete", versionCommand, "-o", "")
	testutil.Equal(t, "completions", result.Stdout, "table\njson\nyaml\n:4\n")
}
