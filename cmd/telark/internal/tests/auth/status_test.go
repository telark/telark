package auth

import (
	"encoding/json"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/auth"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
)

const (
	pinnedAt       = "2026-01-01T00:00:00Z"
	openedAt       = "2026-11-23T10:00:00Z"
	olderOpen      = "2026-11-01T10:00:00Z"
	kindRecover    = "recover"
	stateCompleted = "completed"
	pinned         = "The bootstrap account is test@example.com, pinned since 2026-01-01T00:00:00Z.\n"
)

func pinnedBootstrap(requests ...any) map[string]any {
	return map[string]any{keyEmail: adminEmail, keyPinnedAt: pinnedAt, keyRequests: requests}
}

// Requests are stored newest first, and requests opened before 0.3.0 name no Kubernetes user.
func TestStatusPrintsThePinAndTheNewestRequest(t *testing.T) {
	recovery := map[string]any{keyID: "req-2", keyKind: kindRecover, keyState: "pending", keyOpenedAt: openedAt, keyRequestedBy: testutil.DefaultUser}
	older := map[string]any{keyID: "req-1", keyKind: kindRecover, keyState: stateCompleted, keyOpenedAt: olderOpen}
	replacement := map[string]any{
		keyID: "req-3", keyKind: "replace", keyTarget: "new@example.com", keyState: "canceled", keyReason: "denied", keyOpenedAt: openedAt,
	}
	cases := []struct {
		name      string
		bootstrap map[string]any
		want      string
	}{
		{
			name:      "an open recovery",
			bootstrap: pinnedBootstrap(recovery, older),
			want:      pinned + "Newest request: req-2, recover test@example.com, pending, opened 2026-11-23T10:00:00Z by ops-admin.\n",
		},
		{
			name:      "a canceled replacement from before 0.3.0",
			bootstrap: pinnedBootstrap(replacement),
			want:      pinned + "Newest request: req-3, make new@example.com the bootstrap account, canceled denied, opened 2026-11-23T10:00:00Z.\n",
		},
		{
			name:      "not pinned",
			bootstrap: map[string]any{},
			want:      "The bootstrap account isn't pinned yet: run `telark auth enroll --email <email>`.\nNo request has been opened.\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := testutil.StartCluster(t, testutil.Cluster{})
			fake.Add(t, telarkConfig(c.bootstrap))
			result := testutil.Run(t, authCommand, statusCommand, kubeconfigFlag, kubeconfig(t, fake))
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
			testutil.Equal(t, stdoutLabel, result.Stdout, c.want)
			if writes := fake.Writes(); len(writes) != 0 {
				t.Errorf("writes: got %v, want none", writes)
			}
		})
	}
}

func TestStatusAsJSONOrYAML(t *testing.T) {
	fake := testutil.StartCluster(t, testutil.Cluster{})
	fake.Add(t, telarkConfig(pinnedBootstrap(map[string]any{keyID: "req-1", keyKind: kindRecover, keyState: stateCompleted, keyOpenedAt: openedAt})))
	result := testutil.Run(t, authCommand, statusCommand, kubeconfigFlag, kubeconfig(t, fake), outputFlag, "json")
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
	var got auth.Status
	if err := json.Unmarshal([]byte(result.Stdout), &got); err != nil {
		t.Fatal(err)
	}
	want := auth.Status{Email: adminEmail, PinnedAt: pinnedAt, Pinned: true, NewestRequest: &telarkconfig.BootstrapRequest{
		ID: "req-1", Kind: kindRecover, State: stateCompleted, OpenedAt: openedAt,
	}}
	testutil.Equal(t, "pinned", got.Pinned, want.Pinned)
	testutil.Equal(t, "newest request", *got.NewestRequest, *want.NewestRequest)
	testutil.Equal(t, keyEmail, got.Email, want.Email)
	yaml := testutil.Run(t, authCommand, statusCommand, kubeconfigFlag, kubeconfig(t, fake), outputFlag, "yaml")
	testutil.Contains(t, stdoutLabel, yaml.Stdout, "pinned: true\n")
}

func TestStatusFails(t *testing.T) {
	cases := []struct {
		name    string
		cluster testutil.Cluster
		config  bool
		stdout  bool
		want    string
	}{
		{
			name:    "a release older than 0.3.0",
			cluster: testutil.Cluster{Missing: []string{authRequestsAt}},
			config:  true,
			want: "Error: this Telark serves no AuthRequest objects, so it is older than 0.3.0: run " +
				"`kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email <email>` instead\n",
		},
		{name: "no TelarkConfig", want: `Error: read TelarkConfig default: telarkconfigs.telark.io "default" not found`},
		{name: "stdout fails", config: true, stdout: true, want: "Error: write failed\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := testutil.StartCluster(t, c.cluster)
			if c.config {
				fake.Add(t, telarkConfig(pinnedBootstrap()))
			}
			args := []string{authCommand, statusCommand, kubeconfigFlag, kubeconfig(t, fake)}
			result := testutil.Run(t, args...)
			if c.stdout {
				result = testutil.RunTo(t, testutil.FailingWriter{}, args...)
			}
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Contains(t, stderrLabel, result.Stderr, c.want)
		})
	}
}

func TestStatusNeedsACluster(t *testing.T) {
	t.Setenv(kubeconfigEnv, missingConfig)
	result := testutil.Run(t, authCommand, statusCommand)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
}
