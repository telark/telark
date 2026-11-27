package install

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	exporterEnabled = "exporter.enabled=true"
	exporterName    = "telark-exporter-service"
	oldClaim        = "telark-exporter-snapshots-pvc"
	newClaim        = "telark-exporter-snapshots-pvc-fast"
	movedClaim      = "exporter.claim=" + newClaim
	deployments     = "deployments"
	claims          = "persistentvolumeclaims"
	jobs            = "jobs"
	copyJob         = "telark-storage-copy-00001"
	replicasLabel   = "exporter replicas"
	claimLabel      = "mounted claim"
	jobLabel        = "copy job kept"
	stopped         = "0"
	running         = "1"
	succeeded       = int64(1)
	exporterUser    = int64(10001)
	keySpec         = "spec"
	target          = 1
	exporterPatch   = "PATCH deployments.apps/" + exporterName
	storageMoves    = "‼ The exporter's data moves to new claims (" + oldClaim + " → " + newClaim + "): " +
		"the exporter stops while a Job copies it, before the upgrade\n"
	storageCopying   = "Copying the exporter's data (" + oldClaim + " → " + newClaim + ").\n"
	storageCopied    = "Copied the exporter's data: the old claims are kept.\n"
	storageRetry     = "Error: copy the exporter's data (the exporter runs again on its old claims, and telark upgrade retries the copy): "
	storageUnmanaged = "× The release doesn't hold the Deployment " + exporterName + " on the claims it mounts, " +
		"so the exporter can't be stopped to copy its data\n"
)

func move(t *testing.T, cluster *testutil.FakeCluster, kubeconfig string) <-chan testutil.Result {
	t.Helper()
	var stdout testutil.Stream
	done := testutil.Start(t, &stdout, upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, current, setFlag, movedClaim)
	cluster.Created(t, jobs)
	return done
}

// The fake keeps a number as the encoding it arrived in gave it, so the count is compared as text.
func replicas(cluster *testutil.FakeCluster) string {
	count, _, _ := unstructured.NestedFieldNoCopy(cluster.Find(deployments, exporterName), keySpec, "replicas")
	return fmt.Sprint(count)
}

func mounted(cluster *testutil.FakeCluster) string {
	volumes, _, _ := unstructured.NestedSlice(cluster.Find(deployments, exporterName), keySpec, "template", keySpec, "volumes")
	claim, _, _ := unstructured.NestedString(volumes[none].(map[string]any), "persistentVolumeClaim", "claimName")
	return claim
}

func TestUpgradeCopiesTheExportersDataBeforeItMoves(t *testing.T) {
	cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
	var stdout testutil.Stream
	done := testutil.Start(t, &stdout, upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, current, setFlag, movedClaim)
	testutil.Equal(t, "job name", cluster.Created(t, jobs), copyJob)
	testutil.Equal(t, replicasLabel, replicas(cluster), stopped)
	testutil.Equal(t, claimLabel, mounted(cluster), oldClaim)
	testutil.Equal(t, "new claim's owner", stringMap(cluster.Find(claims, newClaim), "labels")["app.kubernetes.io/managed-by"], "Helm")
	pod, _, _ := unstructured.NestedMap(cluster.Find(jobs, copyJob), keySpec, "template", keySpec)
	var spec corev1.PodSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(pod, &spec); err != nil {
		t.Fatal(err)
	}
	container := spec.Containers[none]
	testutil.Equal(t, "image", container.Image, "ghcr.io/telark/exporter:"+current)
	testutil.Equal(t, "command", strings.Join(append(container.Command, container.Args...), " "),
		"/app/copy-tree /previous/data/snapshots /data/snapshots")
	testutil.Equal(t, "source", spec.Volumes[none].PersistentVolumeClaim.ClaimName, oldClaim)
	testutil.Equal(t, "source is read-only", spec.Volumes[none].PersistentVolumeClaim.ReadOnly, true)
	testutil.Equal(t, "target", spec.Volumes[target].PersistentVolumeClaim.ClaimName, newClaim)
	testutil.Equal(t, "runs as", *spec.SecurityContext.RunAsUser, exporterUser)
	cluster.SetStatus(t, jobs, copyJob, map[string]any{"succeeded": succeeded})
	result := testutil.Wait(t, done)
	testutil.Equal(t, stderrLabel, result.Stderr, waitingMessage)
	testutil.Equal(t, codeLabel, result.Code, passed)
	for _, part := range []string{storageMoves, storageCopying + storageCopied + "Upgraded Telark to 0.2.1"} {
		testutil.Contains(t, stdoutLabel, stdout.String(), part)
	}
	testutil.Equal(t, replicasLabel, replicas(cluster), running)
	testutil.Equal(t, claimLabel, mounted(cluster), newClaim)
	again := upgrade(t, kubeconfig, current, dryRunFlag)
	testutil.Equal(t, "second upgrade", strings.Contains(again.Stdout, "moves to new claims"), false)
}

func TestUpgradeDryRunNamesTheMove(t *testing.T) {
	cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
	result := upgrade(t, kubeconfig, current, dryRunFlag, setFlag, movedClaim)
	testutil.Contains(t, stdoutLabel, result.Stdout, storageMoves)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, replicasLabel, replicas(cluster), running)
	testutil.Equal(t, "no new claim", cluster.Find(claims, newClaim) == nil, true)
}

func TestUpgradeWithoutAMove(t *testing.T) {
	unmanaged := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: exporterName, Namespace: namespace, Labels: map[string]string{"app.kubernetes.io/managed-by": "Helm"},
			Annotations: map[string]string{"meta.helm.sh/release-name": "telark", "meta.helm.sh/release-namespace": namespace},
		},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: []corev1.Volume{{
			Name: "snapshots-storage", VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: newClaim}},
		}}}}},
	}
	cases := map[string]storageCase{
		"the same claims":              {installed: []string{setFlag, exporterEnabled}, args: []string{setFlag, exporterEnabled}},
		"an exporter the upgrade adds": {args: []string{setFlag, exporterEnabled}},
		"an unreadable exporter": {
			installed: []string{setFlag, exporterEnabled}, args: []string{setFlag, movedClaim}, code: failed,
			refused: "GET deployments.apps/" + exporterName, line: "× Couldn't read the exporter's Deployment: ",
		},
		"an exporter the release doesn't hold": {
			objects: []runtime.Object{unmanaged}, args: []string{setFlag, exporterEnabled}, code: failed, line: storageUnmanaged,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, previous, c.installed...)
			cluster.Add(t, c.objects...)
			if c.refused != "" {
				cluster.Refuse(c.refused, http.StatusInternalServerError)
			}
			result := upgrade(t, kubeconfig, current, append([]string{dryRunFlag}, c.args...)...)
			testutil.Contains(t, stdoutLabel, result.Stdout, c.line)
			testutil.Equal(t, "names a move", strings.Contains(result.Stdout, "moves to new claims"), false)
			testutil.Equal(t, codeLabel, result.Code, c.code)
		})
	}
}

func TestUpgradeStopsBeforeTheCopy(t *testing.T) {
	cases := map[string]stopCase{
		"claims the server refuses": {
			change: "PATCH persistentvolumeclaims/" + newClaim, stderr: "Error: create the exporter's new claims: ", replicas: running,
		},
		"an exporter that can't be stopped": {change: exporterPatch, stderr: "Error: stop the exporter: ", replicas: running},
		"pods that can't be listed":         {refused: "GET pods/", stderr: storageRetry, replicas: running},
		"a Job the server refuses":          {refused: "POST jobs.batch/", stderr: storageRetry, replicas: running},
		"a Job that can't be read":          {refused: "GET jobs.batch/" + copyJob, stderr: storageRetry, replicas: running},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
			if c.refused != "" {
				cluster.Refuse(c.refused, http.StatusForbidden)
			}
			if c.change != "" {
				cluster.RefuseChange(c.change, http.StatusForbidden)
			}
			result := upgrade(t, kubeconfig, current, setFlag, movedClaim)
			testutil.Contains(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, failed)
			testutil.Equal(t, replicasLabel, replicas(cluster), c.replicas)
			testutil.Equal(t, claimLabel, mounted(cluster), oldClaim)
			testutil.Equal(t, "no copy left running", cluster.Find(jobs, copyJob) == nil, true)
			testutil.Equal(t, secondLabel, cluster.Find(secrets, secondRelease) == nil, true)
		})
	}
}

func TestUpgradeKeepsAFailedCopyAndRestartsTheExporter(t *testing.T) {
	cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
	done := move(t, cluster, kubeconfig)
	cluster.SetStatus(t, jobs, copyJob, map[string]any{"conditions": []any{map[string]any{"type": "Failed", "status": "True"}}})
	result := testutil.Wait(t, done)
	testutil.Contains(t, stderrLabel, result.Stderr, storageRetry+"the copy Job failed: read kubectl logs -n telark job/"+copyJob+"\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
	testutil.Equal(t, replicasLabel, replicas(cluster), running)
	testutil.Equal(t, claimLabel, mounted(cluster), oldClaim)
	testutil.Equal(t, jobLabel, cluster.Find(jobs, copyJob) != nil, true)
}

func TestUpgradeFailureAfterTheCopy(t *testing.T) {
	cases := map[string]stopCase{
		"before Helm reaches the exporter": {refused: "POST secrets/", replicas: running, claim: oldClaim},
		"after Helm applied the exporter":  {refused: "PUT secrets/" + secondRelease, replicas: running, claim: newClaim},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
			cluster.Refuse(c.refused, http.StatusInternalServerError)
			done := move(t, cluster, kubeconfig)
			cluster.SetStatus(t, jobs, copyJob, map[string]any{"succeeded": succeeded})
			result := testutil.Wait(t, done)
			testutil.Contains(t, stderrLabel, result.Stderr, "Error: upgrade: ")
			testutil.Equal(t, codeLabel, result.Code, failed)
			testutil.Equal(t, replicasLabel, replicas(cluster), c.replicas)
			testutil.Equal(t, claimLabel, mounted(cluster), c.claim)
		})
	}
}

func TestUpgradeRetryAfterAFailedUpgradeKeepsTheOldClaims(t *testing.T) {
	cluster, kubeconfig := released(t, previous, setFlag, exporterEnabled)
	cluster.RefuseChange("PATCH configmaps/telark-extra", http.StatusInternalServerError)
	var stdout testutil.Stream
	done := testutil.Start(t, &stdout, upgradeCommand, kubeconfigFlag, kubeconfig, versionFlag, current,
		setFlag, movedClaim, setFlag, extraEnabled)
	cluster.SetStatus(t, jobs, cluster.Created(t, jobs), map[string]any{"succeeded": succeeded})
	testutil.Contains(t, stderrLabel, testutil.Wait(t, done).Stderr, "Error: upgrade: ")
	testutil.Equal(t, releaseLabel, status(cluster, secondRelease), "failed")
	testutil.Equal(t, claimLabel, mounted(cluster), oldClaim)
	cluster.Refuse("POST jobs.batch/", http.StatusForbidden)
	retry := upgrade(t, kubeconfig, current, setFlag, movedClaim, setFlag, extraEnabled)
	testutil.Contains(t, stderrLabel, retry.Stderr, storageRetry)
	testutil.Equal(t, replicasLabel, replicas(cluster), running)
	testutil.Equal(t, claimLabel, mounted(cluster), oldClaim)
}
