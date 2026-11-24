package check

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	outputFlag   = "-o"
	waitFlag     = "--wait"
	checkCount   = 10
	kyvernoIndex = 4
	firstIndex   = 0
	shortWait    = 300 * time.Millisecond
	longestRun   = 30 * time.Second
	noWait       = 0
)

func TestCheckFlagErrors(t *testing.T) {
	cases := map[string]struct {
		args   []string
		stderr string
	}{
		"--mode without --pre": {[]string{modeFlag, "minimal"}, "Error: --mode works only with --pre\n"},
		"an unknown mode":      {[]string{preFlag, modeFlag, "tiny"}, `Error: --mode "tiny": must be minimal, standard or performance` + "\n"},
		"an argument":          {[]string{"extra"}, `Error: unknown command "extra" for "telark check"` + "\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			result := run(t, testutil.Cluster{}, c.args...)
			testutil.Equal(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, failed)
		})
	}
}

func TestCheckWithoutACluster(t *testing.T) {
	t.Setenv("KUBECONFIG", t.TempDir()+"/missing")
	result := testutil.Run(t, checkCommand)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestCheckFailureSummary(t *testing.T) {
	result := run(t, objects(without(testutil.HealthyTelark(), kyvernoWebhook, "sessions.telark.io")))
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: 2 of 10 checks failed\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestCheckJSON(t *testing.T) {
	result := run(t, objects(without(testutil.HealthyTelark(), kyvernoWebhook)), outputFlag, "json")
	var report check.Report
	if err := json.Unmarshal([]byte(result.Stdout), &report); err != nil {
		t.Fatalf("%v in %q", err, result.Stdout)
	}
	testutil.Equal(t, "success", report.Success, false)
	testutil.Equal(t, "checks", len(report.Checks), checkCount)
	testutil.Equal(t, "first", report.Checks[firstIndex],
		check.Result{Name: "crds", Status: "pass", Message: "10 CRDs are established (telark-crds 0.2.1)"})
	testutil.Equal(t, "kyverno", report.Checks[kyvernoIndex],
		check.Result{Name: "kyverno", Status: "fail", Message: "Kyverno has no webhooks registered, so no protection plan is enforced"})
	testutil.Equal(t, "last", report.Checks[checkCount-1].Name, "update")
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: 1 of 10 checks failed\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestCheckYAML(t *testing.T) {
	result := run(t, objects(testutil.HealthyTelark()), outputFlag, "yaml", skipFlag)
	testutil.Contains(t, stdoutLabel, result.Stdout, "success: true\n")
	testutil.Contains(t, stdoutLabel, result.Stdout, "- message: The CRD guard is enforced\n  name: crd-guard\n  status: pass\n")
	testutil.Equal(t, codeLabel, result.Code, passed)
}

func TestCheckWriteFailure(t *testing.T) {
	kubeconfig := testutil.Kubeconfig(t, contextName, map[string]string{contextName: testutil.ClusterServer(t, objects(testutil.HealthyTelark()))})
	result := testutil.RunTo(t, testutil.FailingWriter{}, checkCommand, "--kubeconfig", kubeconfig, outputFlag, "json", skipFlag)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: write failed\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestCheckWait(t *testing.T) {
	cases := map[string]struct {
		objects []runtime.Object
		wait    string
		atLeast time.Duration
		stderr  string
		code    int
	}{
		"checks that pass at once": {
			objects: testutil.HealthyTelark(), wait: "1m", atLeast: noWait, stderr: "Waiting up to 1m0s for every check to pass\n", code: passed,
		},
		"checks that keep failing": {
			objects: without(testutil.HealthyTelark(), kyvernoWebhook), wait: shortWait.String(), atLeast: shortWait,
			stderr: "Waiting up to 300ms for every check to pass\nError: 1 of 10 checks failed\n", code: failed,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			github.Answer(t, http.StatusOK, testutil.LatestFound, nil)
			started := time.Now()
			result := run(t, objects(c.objects), waitFlag, c.wait)
			testutil.Equal(t, "requests", len(github.Requests), oneRequest)
			if elapsed := time.Since(started); elapsed < c.atLeast || elapsed > longestRun {
				t.Errorf("elapsed: got %v, want between %v and %v", elapsed, c.atLeast, longestRun)
			}
			testutil.Equal(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func TestCheckCompletesModes(t *testing.T) {
	result := testutil.Run(t, "__complete", checkCommand, preFlag, modeFlag, "")
	testutil.Equal(t, "completions", result.Stdout, "minimal\nstandard\nperformance\n:4\n")
}
