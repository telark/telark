package check

import (
	"slices"
	"testing"
	"time"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	restarts      = 3
	recently      = 10 * time.Minute
	longAgo       = 2 * time.Hour
	guardOff      = "‼ The CRD guard is off"
	failed        = constants.ExitCodeFailure
	passed        = constants.ExitCodeSuccess
	healthyOutput = `✔ 10 CRDs are established (telark-crds 0.2.1)
✔ Every Deployment (3) and StatefulSet (1) is available
✔ No pod restarted in the last hour
✔ The CLI (0.0.0-dev) and the server (0.0.0-dev) are from the same release
✔ Kyverno's admission controller is available and its webhooks are registered
✔ The CRD guard is enforced
✔ The bootstrap account test@example.com is pinned
✔ No recovery request is open
✔ No protection plan is drifted or degraded
✔ This CLI (0.0.0-dev) is the latest release
`
)

func TestCheckPassesOnAHealthyRelease(t *testing.T) {
	result := run(t, testutil.Cluster{Objects: testutil.HealthyTelark()})
	testutil.Equal(t, stdoutLabel, result.Stdout, healthyOutput)
	testutil.Equal(t, stderrLabel, result.Stderr, "")
	testutil.Equal(t, codeLabel, result.Code, passed)
}

func TestCheckCRDs(t *testing.T) {
	expect(t, map[string]checkCase{
		"a missing CRD": {
			cluster: objects(without(testutil.HealthyTelark(), "sessions.telark.io")),
			lines:   []string{"× CRDs missing or not established: sessions.telark.io\n"}, code: failed,
		},
		"a CRD not established": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.CRD("users.telark.io", crdVersion, false))),
			lines:   []string{"× CRDs missing or not established: users.telark.io\n"}, code: failed,
		},
		"CRDs from two charts": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.CRD("users.telark.io", "0.3.0", true))),
			lines:   []string{"‼ The CRDs come from several chart versions: 0.2.1, 0.3.0\n"},
		},
		"one CRD without the chart's labels": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.CRD("users.telark.io", "", true))),
			lines:   []string{"‼ The CRDs come from several chart versions: 0.2.1, unknown\n"},
		},
		"CRDs without the chart's labels": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.CRDs("")...)),
			lines:   []string{"✔ 10 CRDs are established\n"},
		},
		"unreadable CRDs": {
			cluster: forbidden("customresourcedefinitions.apiextensions.k8s.io"),
			lines:   []string{"× Couldn't read the CRDs: "}, code: failed,
		},
	})
}

func TestCheckWorkloads(t *testing.T) {
	expect(t, map[string]checkCase{
		"no Telark in the namespace": {
			cluster: objects(testutil.HealthyTelark()), args: []string{"-n", "other"},
			lines: []string{
				`× Couldn't read the workloads: no Telark deployments in namespace "other": pass the namespace Telark runs in with -n` + "\n",
				"× TelarkConfig default doesn't exist yet: the exporter creates it when it starts\n",
			},
			code: failed,
		},
		"unreadable Deployments": {
			cluster: forbidden("deployments.apps"),
			lines: []string{
				`× Couldn't read the workloads: list the Telark deployments in namespace "telark": `,
				"× Couldn't read Kyverno's admission controller: ",
			},
			code: failed,
		},
		"a Deployment down": {
			cluster: objects(with(testutil.HealthyTelark(), downExporter())),
			lines:   []string{"× Not available: exporter-service (0 of 1)\n"}, code: failed,
		},
		"a StatefulSet down": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.StatefulSet(namespace, redis, down))),
			lines:   []string{"× Not available: telark-redis-master (0 of 1)\n"}, code: failed,
		},
		"unreadable StatefulSets": {
			cluster: forbidden("statefulsets.apps"), lines: []string{"× Couldn't read the workloads: "}, code: failed,
		},
		"a pod restarted in the last hour": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.Pod(namespace, authPod, restarts, time.Now().Add(-recently), "OOMKilled"))),
			lines:   []string{"‼ Pods restarted in the last hour: telark-auth-service-0 (3, OOMKilled)\n"},
		},
		"a pod restarted before the last hour": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.Pod(namespace, authPod, restarts, time.Now().Add(-longAgo), "Error"))),
			lines:   []string{"✔ No pod restarted in the last hour\n"},
		},
		"unreadable pods": {
			cluster: forbidden("pods"), lines: []string{"× Couldn't read the pods: "}, code: failed,
		},
		"a server from another release": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.ExporterDeployment("0.2.3"))),
			lines: []string{
				"‼ The CLI (0.0.0-dev) and the server (0.0.0-dev, 0.2.3) are from different releases: use the CLI that matches the server\n",
			},
		},
	})
}

func TestCheckAdmission(t *testing.T) {
	expect(t, map[string]checkCase{
		"no Kyverno webhooks": {
			cluster: objects(without(testutil.HealthyTelark(), kyvernoWebhook)),
			lines:   []string{"× Kyverno has no webhooks registered, so no protection plan is enforced\n"}, code: failed,
		},
		"unreadable webhooks": {
			cluster: forbidden("validatingwebhookconfigurations.admissionregistration.k8s.io"),
			lines:   []string{"× Couldn't read Kyverno's webhooks: "}, code: failed,
		},
		"Kyverno's admission controller down": {
			cluster: objects(with(testutil.HealthyTelark(), downController())),
			lines:   []string{"× Kyverno's admission controller isn't available\n"}, code: failed,
		},
		"no CRD guard": {
			cluster: objects(without(testutil.HealthyTelark(), guardPolicy)),
			lines:   []string{guardOff + ", so any account that may write Telark's resources can change them directly\n"},
		},
		"no CRD guard binding": {
			cluster: objects(without(testutil.HealthyTelark(), guardBinding)), lines: []string{guardOff},
		},
		"a CRD guard that audits": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.GuardBinding(admissionregistrationv1.Audit, admissionregistrationv1.Warn))),
			lines:   []string{"‼ The CRD guard only audits: set app.security.crdGuard.enforce=true to enforce it\n"},
		},
		"an unreadable CRD guard": {
			cluster: forbidden("validatingadmissionpolicies.admissionregistration.k8s.io"),
			lines:   []string{"× Couldn't read the CRD guard: "}, code: failed,
		},
		"an unreadable CRD guard binding": {
			cluster: forbidden("validatingadmissionpolicybindings.admissionregistration.k8s.io"),
			lines:   []string{"× Couldn't read the CRD guard: "}, code: failed,
		},
	})
}

func TestCheckBootstrap(t *testing.T) {
	expect(t, map[string]checkCase{
		"an unreadable TelarkConfig": {
			cluster: forbidden("telarkconfigs.telark.io"), lines: []string{"× Couldn't read TelarkConfig default: "}, code: failed,
		},
		"a malformed TelarkConfig": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig("pinned"))),
			lines:   []string{"× Couldn't read TelarkConfig default: json: cannot unmarshal string"}, code: failed,
		},
		"an unpinned bootstrap account": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig(map[string]any{}))),
			lines:   []string{"‼ The bootstrap account isn't pinned yet: run `telark auth enroll --email test@example.com`\n"},
		},
		"an unpinned bootstrap account and no admin in the exporter's env": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig(map[string]any{}), testutil.ExporterDeployment(cliVersion))),
			lines:   []string{"run `telark auth enroll --email <email>`\n"},
		},
		"an open recovery request": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig(requests("pending", pinnedAt)))),
			lines:   []string{"‼ Request r1 (recover) is open and can complete at 2026-01-03T00:00:00Z\n"},
		},
		"an approved request and the default wait": {
			cluster: objects(with(testutil.HealthyTelark(),
				testutil.TelarkConfig(requests("approved", pinnedAt)), testutil.ExporterDeployment(cliVersion))),
			lines: []string{"‼ Request r1 (recover) is open and can complete at 2026-01-02T00:00:00Z\n"},
		},
		"an open request with a malformed time": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig(requests("pending", "yesterday")))),
			lines:   []string{"‼ Request r1 (recover) is open and can complete at unknown\n"},
		},
		"a finished request": {
			cluster: objects(with(testutil.HealthyTelark(), testutil.TelarkConfig(requests("completed", pinnedAt)))),
			lines:   []string{"✔ No recovery request is open\n"},
		},
	})
}

func TestCheckProtectionPlans(t *testing.T) {
	expect(t, map[string]checkCase{
		"drifted and degraded plans": {
			cluster: objects(with(testutil.HealthyTelark(),
				testutil.ProtectionPlan("plan-a", "drifted"), testutil.ProtectionPlan("plan-b", "degraded"))),
			lines: []string{"‼ Protection plans drifted or degraded (2): plan-a, plan-b\n"},
		},
		"unreadable plans": {
			cluster: forbidden("protectionplans.telark.io"), lines: []string{"× Couldn't read the protection plans: "}, code: failed,
		},
	})
}

func expect(t *testing.T, cases map[string]checkCase, flags ...string) {
	t.Helper()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			result := run(t, c.cluster, slices.Concat([]string{skipFlag}, flags, c.args)...)
			for _, line := range c.lines {
				testutil.Contains(t, stdoutLabel, result.Stdout, line)
			}
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func objects(list []runtime.Object) testutil.Cluster {
	return testutil.Cluster{Objects: list}
}

func forbidden(resource string) testutil.Cluster {
	return testutil.Cluster{Objects: testutil.HealthyTelark(), Forbidden: []string{resource}}
}

func downExporter() runtime.Object {
	deployment := testutil.ExporterDeployment(cliVersion)
	deployment.Status.AvailableReplicas = down
	return deployment
}

func downController() runtime.Object {
	deployment := testutil.Deployment(namespace, controller, "telark-kyverno", "3.9.2", "reg.kyverno.io/kyverno/kyverno:v1.19.2")
	deployment.Labels[nameLabel] = kyvernoName
	deployment.Status.AvailableReplicas = down
	return deployment
}

func requests(state, openedAt string) map[string]any {
	return map[string]any{"email": adminEmail, "pinnedAt": pinnedAt, "requests": []any{
		map[string]any{"id": "r1", "kind": "recover", "state": state, "openedAt": openedAt},
	}}
}
