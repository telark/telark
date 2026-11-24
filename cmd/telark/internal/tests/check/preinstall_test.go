package check

import (
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	preFlag     = "--pre"
	modeFlag    = "--mode"
	nodeName    = "node-1"
	classes     = "standard"
	system      = "kube-system"
	twoCPUs     = "2"
	eightCPUs   = "8"
	oneGiB      = "1Gi"
	bigMemory   = "16Gi"
	snapshots   = "telark-exporter-snapshots-pvc"
	freshOutput = `✔ Kubernetes v1.34.0 is 1.30 or newer
✔ You can create the 20 kinds of objects the chart installs
✔ standard is the default StorageClass, for the volume claims
✔ No other Kyverno runs in the cluster
✔ No data from an earlier install
✔ The nodes have 4.00 CPU and 8.0 GiB free, enough for the standard mode's 1.65 CPU and 3.3 GiB
✔ This CLI (0.0.0-dev) is the latest release
`
)

func TestCheckPrePassesOnAFreshCluster(t *testing.T) {
	result := run(t, testutil.Cluster{Objects: testutil.FreshCluster()}, preFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, freshOutput)
	testutil.Equal(t, codeLabel, result.Code, passed)
}

func TestCheckPreAccess(t *testing.T) {
	expect(t, map[string]checkCase{
		"an old Kubernetes": {
			cluster: testutil.Cluster{Objects: testutil.FreshCluster(), Version: "v1.29.3"},
			lines:   []string{"× Kubernetes v1.29.3 is older than 1.30, the oldest release Telark supports\n"}, code: failed,
		},
		"an unreadable server version": {
			cluster: blocked("version"), lines: []string{"× Couldn't read the server version: "}, code: failed,
		},
		"a server version that isn't one": {
			cluster: testutil.Cluster{Objects: testutil.FreshCluster(), Version: "devel"},
			lines:   []string{"× Couldn't read the server version: "}, code: failed,
		},
		"an existing namespace": {
			cluster: objects(with(testutil.FreshCluster(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})),
			lines:   []string{"✔ You can create the 19 kinds of objects the chart installs\n"},
		},
		"missing permissions": {
			cluster: testutil.Cluster{
				Objects: testutil.FreshCluster(), Denied: []string{"customresourcedefinitions.apiextensions.k8s.io", "secrets"},
			},
			lines: []string{"× You can't create customresourcedefinitions.apiextensions.k8s.io, secrets, which the chart installs\n"}, code: failed,
		},
		"unreadable permissions": {
			cluster: blocked("selfsubjectaccessreviews.authorization.k8s.io"), lines: []string{"× Couldn't read your permissions: "}, code: failed,
		},
	}, preFlag)
}

func TestCheckPreStorageAndKyverno(t *testing.T) {
	expect(t, map[string]checkCase{
		"no default StorageClass": {
			cluster: objects(with(testutil.FreshCluster(), testutil.StorageClass(classes, nil))),
			lines: []string{"× No default StorageClass, so the volume claims would stay Pending: " +
				"mark a class as the default, or set app.persistence.storageClass\n"},
			code: failed,
		},
		"a default StorageClass by the beta annotation": {
			cluster: objects(with(testutil.FreshCluster(), testutil.StorageClass(classes,
				map[string]string{"storageclass.beta.kubernetes.io/is-default-class": "true"}))),
			lines: []string{"✔ standard is the default StorageClass, for the volume claims\n"},
		},
		"unreadable StorageClasses": {
			cluster: blocked("storageclasses.storage.k8s.io"), lines: []string{"× Couldn't read the StorageClasses: "}, code: failed,
		},
		"another Kyverno": {
			cluster: objects(with(testutil.FreshCluster(), testutil.KyvernoWebhook(kyvernoWebhook))),
			lines: []string{"× Kyverno already runs in this cluster (kyverno-resource-validating-webhook-cfg): " +
				"to use it, install with kyverno.enabled=false\n"},
			code: failed,
		},
		"unreadable webhooks": {
			cluster: blocked("validatingwebhookconfigurations.admissionregistration.k8s.io"),
			lines:   []string{"× Couldn't read Kyverno's webhooks: "}, code: failed,
		},
	}, preFlag)
}

func TestCheckPreEarlierInstall(t *testing.T) {
	expect(t, map[string]checkCase{
		"Telark already running": {
			cluster: objects(with(testutil.FreshCluster(),
				testutil.Deployment("elsewhere", exporter, partOf, "0.2.3", "ghcr.io/telark/exporter:0.2.3"))),
			lines: []string{"× Telark 0.2.3 already runs in namespace \"elsewhere\": upgrade it instead of installing\n"}, code: failed,
		},
		"unreadable Deployments": {
			cluster: blocked("deployments.apps"), lines: []string{"× Couldn't read Telark's Deployments: "}, code: failed,
		},
		"CRDs and claims from an earlier install": {
			cluster: objects(with(testutil.FreshCluster(), testutil.CRD("users.telark.io", crdVersion, true),
				testutil.CRD("policies.kyverno.io", crdVersion, true), claim(snapshots))),
			lines: []string{"‼ An earlier install left Telark's CRDs (1) and volume claims (1) in namespace \"telark\": " +
				"a reinstall keeps this data\n"},
		},
		"claims from an earlier install": {
			cluster: objects(with(testutil.FreshCluster(), claim(snapshots))),
			lines:   []string{"‼ An earlier install left volume claims (1) in namespace \"telark\": a reinstall keeps this data\n"},
		},
		"unreadable CRDs": {
			cluster: blocked("customresourcedefinitions.apiextensions.k8s.io"), lines: []string{"× Couldn't read the CRDs: "}, code: failed,
		},
		"unreadable claims": {
			cluster: blocked("persistentvolumeclaims"), lines: []string{"× Couldn't read the volume claims: "}, code: failed,
		},
	}, preFlag)
}

func TestCheckPreCapacity(t *testing.T) {
	expect(t, map[string]checkCase{
		"too little capacity": {
			cluster: objects(with(testutil.FreshCluster(), testutil.Node(nodeName, "1", "2Gi", true))),
			lines: []string{"‼ The nodes have 1.00 CPU and 2.0 GiB free, and the standard mode requests 1.65 CPU and 3.3 GiB: " +
				"some pods stay Pending until the cluster adds nodes\n"},
		},
		"capacity the running pods use": {
			cluster: objects(with(testutil.FreshCluster(),
				testutil.RunningPod(system, "busy", nodeName, "3", oneGiB, corev1.PodRunning),
				testutil.RunningPod(system, "done", nodeName, twoCPUs, oneGiB, corev1.PodSucceeded),
				testutil.RunningPod(system, "gone", nodeName, twoCPUs, oneGiB, corev1.PodFailed),
				testutil.RunningPod(system, "unscheduled", "", twoCPUs, oneGiB, corev1.PodPending))),
			lines: []string{"‼ The nodes have 1.00 CPU and 7.0 GiB free, and the standard mode requests 1.65 CPU and 3.3 GiB"},
		},
		"nodes a pod without tolerations can't use": {
			cluster: objects(append(testutil.FreshCluster(), unusableNodes()...)),
			lines:   []string{"✔ The nodes have 5.00 CPU and 10.0 GiB free, enough for the standard mode's 1.65 CPU and 3.3 GiB\n"},
		},
		"the minimal mode": {
			cluster: objects(with(testutil.FreshCluster(), testutil.Node(nodeName, "1500m", "3Gi", true))), args: []string{modeFlag, "minimal"},
			lines: []string{"✔ The nodes have 1.50 CPU and 3.0 GiB free, enough for the minimal mode's 1.35 CPU and 2.9 GiB\n"},
		},
		"the performance mode": {
			cluster: objects(with(testutil.FreshCluster(), testutil.Node(nodeName, twoCPUs, "4Gi", true))), args: []string{modeFlag, "performance"},
			lines: []string{"‼ The nodes have 2.00 CPU and 4.0 GiB free, and the performance mode requests 2.55 CPU and 5.6 GiB"},
		},
		"unreadable nodes": {
			cluster: blocked("nodes"), lines: []string{"× Couldn't read the nodes: "}, code: failed,
		},
		"unreadable pods": {
			cluster: blocked("pods"), lines: []string{"× Couldn't read the pods: "}, code: failed,
		},
	}, preFlag)
}

func blocked(resource string) testutil.Cluster {
	return testutil.Cluster{Objects: testutil.FreshCluster(), Forbidden: []string{resource}}
}

func claim(name string) runtime.Object {
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

func unusableNodes() []runtime.Object {
	cordoned := testutil.Node("cordoned", eightCPUs, bigMemory, true)
	cordoned.Spec.Unschedulable = true
	unknown := testutil.Node("unknown", eightCPUs, bigMemory, true)
	unknown.Status.Conditions = nil
	return []runtime.Object{
		cordoned,
		unknown,
		testutil.Node("not-ready", eightCPUs, bigMemory, false),
		testutil.Node("control-plane", eightCPUs, bigMemory, true,
			corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}),
		testutil.Node("draining", eightCPUs, bigMemory, true, corev1.Taint{Key: "drain", Effect: corev1.TaintEffectNoExecute}),
		testutil.Node("preferred", "1", "2Gi", true, corev1.Taint{Key: "spot", Effect: corev1.TaintEffectPreferNoSchedule}),
	}
}
