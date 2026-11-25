package install

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	uninstallCommand = "uninstall"
	yesFlag          = "--yes"
	timeoutFlag      = "--timeout"
	users            = "users"
	groups           = "groups"
	roles            = "accessroles"
	userName         = "release_lead"
	groupName        = "platform_team"
	roleName         = "plan_approver"
	userCleanup      = "telark.io/user-cleanup"
	otherFinalizer   = "example.com/audit"
	userPatch        = "PATCH users.telark.io/" + userName
	finalizersLabel  = "finalizers"
	oneLeft          = 1
	untouched        = 2
	uninstallPlan    = "The release telark (0.2.1) in namespace \"telark\" will be uninstalled.\n" + heldThree
	heldThree        = "3 users, groups and access roles hold a cleanup finalizer, which will be removed.\n" + keeps
	keeps            = "The CRDs, the custom resources, the Secrets and the volumes are kept.\n"
	noRelease        = "No release telark in namespace \"telark\": only the cleanup finalizers are removed.\n"
	confirm          = "Proceed? [y/N]: "
	uninstalled      = "Uninstalled the release telark.\n"
	removedThree     = "Cleanup finalizers: 3 removed, 0 failed.\n" + teardown
	teardown         = "To delete Telark's data too, see \"Full teardown\" in docs/INSTALL.md.\n"
	notInstalled     = "Telark isn't installed in namespace \"telark\": no release and no cleanup finalizer.\n"
	notSure          = "Error: uninstall canceled: answer y, or pass --yes\n"
	left             = "cleanup finalizers are left: clear them with the kubectl command under \"Uninstall\" in docs/INSTALL.md\n"
)

func held(kind, name string, finalizers ...string) runtime.Object {
	object := testutil.Resource("telark.io/v1alpha1", kind, namespace, name, nil)
	object.SetFinalizers(finalizers)
	return object
}

func accounts() []runtime.Object {
	return []runtime.Object{
		held("User", userName, userCleanup, otherFinalizer), held("User", "auditor"),
		held("Group", groupName, "telark.io/group-cleanup"), held("AccessRole", roleName, "telark.io/role-cleanup"),
	}
}

func uninstall(t *testing.T, kubeconfig, input string, args ...string) testutil.Result {
	t.Helper()
	var stdout bytes.Buffer
	return testutil.RunInput(t, input, &stdout, append([]string{uninstallCommand, kubeconfigFlag, kubeconfig}, args...)...)
}

func finalizers(cluster *testutil.FakeCluster, resource, name string) []string {
	list, _, _ := unstructured.NestedStringSlice(cluster.Find(resource, name), "metadata", finalizersLabel)
	return list
}

func TestUninstall(t *testing.T) {
	cases := map[string]uninstallCase{
		"confirmed with --yes": {args: []string{yesFlag}, stdout: uninstallPlan + uninstalled + removedThree},
		"confirmed at the prompt": {
			input: "Y\n", stdout: uninstallPlan + confirm + uninstalled + removedThree,
		},
		"waiting for the objects to go": {args: []string{yesFlag, waitFlag}, stdout: uninstallPlan + uninstalled + removedThree},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, current)
			cluster.Add(t, accounts()...)
			result := uninstall(t, kubeconfig, c.input, c.args...)
			testutil.Equal(t, stdoutLabel, result.Stdout, c.stdout)
			testutil.Equal(t, stderrLabel, result.Stderr, "")
			testutil.Equal(t, codeLabel, result.Code, passed)
			testutil.Equal(t, releaseLabel, cluster.Find(secrets, firstRelease) == nil, true)
			testutil.Equal(t, "settings", cluster.Find(configMaps, settings) == nil, true)
			kept := finalizers(cluster, users, userName)
			testutil.Equal(t, "kept finalizers", len(kept), oneLeft)
			testutil.Equal(t, "kept finalizer", kept[none], otherFinalizer)
			testutil.Equal(t, "group", len(finalizers(cluster, groups, groupName)), none)
			testutil.Equal(t, "role", len(finalizers(cluster, roles, roleName)), none)
			again := uninstall(t, kubeconfig, "", yesFlag)
			testutil.Equal(t, "second run", again.Stdout, notInstalled)
			testutil.Equal(t, "second code", again.Code, passed)
		})
	}
}

func TestUninstallChangesNothing(t *testing.T) {
	cases := map[string]uninstallCase{
		"on a dry run":          {args: []string{dryRunFlag}, stdout: uninstallPlan + "Dry run: nothing changed.\n"},
		"without an answer":     {stdout: uninstallPlan + confirm, stderr: notSure, code: failed},
		"when the answer is no": {input: "n\n", stdout: uninstallPlan + confirm, stderr: notSure, code: failed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, current)
			cluster.Add(t, accounts()...)
			result := uninstall(t, kubeconfig, c.input, c.args...)
			testutil.Equal(t, stdoutLabel, result.Stdout, c.stdout)
			testutil.Equal(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, codeLabel, result.Code, c.code)
			testutil.Equal(t, releaseLabel, status(cluster, firstRelease), deployed)
			testutil.Equal(t, finalizersLabel, len(finalizers(cluster, users, userName)), untouched)
		})
	}
}

func TestUninstallAfterHelmUninstall(t *testing.T) {
	cluster, kubeconfig := start(t, accounts()...)
	result := uninstall(t, kubeconfig, "", yesFlag)
	testutil.Equal(t, stdoutLabel, result.Stdout, noRelease+heldThree+removedThree)
	testutil.Equal(t, codeLabel, result.Code, passed)
	testutil.Equal(t, finalizersLabel, len(finalizers(cluster, users, userName)), oneLeft)
}

func TestUninstallWithoutTelark(t *testing.T) {
	_, kubeconfig := start(t)
	result := uninstall(t, kubeconfig, "")
	testutil.Equal(t, stdoutLabel, result.Stdout, notInstalled)
	testutil.Equal(t, codeLabel, result.Code, passed)
	failing := testutil.RunTo(t, testutil.FailingWriter{}, uninstallCommand, kubeconfigFlag, kubeconfig)
	testutil.Equal(t, stderrLabel, failing.Stderr, writeFailed)
	testutil.Equal(t, codeLabel, failing.Code, failed)
}

func TestUninstallDryRunWriteFailure(t *testing.T) {
	_, kubeconfig := released(t, current)
	result := testutil.RunTo(t, testutil.FailingWriter{}, uninstallCommand, kubeconfigFlag, kubeconfig, dryRunFlag)
	testutil.Equal(t, stderrLabel, result.Stderr, writeFailed)
	testutil.Equal(t, codeLabel, result.Code, failed)
}

func TestUninstallFinalizerFailures(t *testing.T) {
	cases := map[string]uninstallCase{
		"a kind that can't be listed": {
			refused: map[string]int32{"GET users.telark.io/": http.StatusForbidden}, code: failed,
			stdout: "Cleanup finalizers: 2 removed, 1 failed.\n", stderr: "Couldn't list users: ", finalizers: untouched,
		},
		"a kind whose CRD is gone": {
			refused: map[string]int32{"GET users.telark.io/": http.StatusNotFound},
			stdout:  "Cleanup finalizers: 2 removed, 0 failed.\n", finalizers: untouched,
		},
		"a patch the server refuses": {
			refused: map[string]int32{userPatch: http.StatusForbidden}, code: failed,
			stdout: "Cleanup finalizers: 2 removed, 1 failed.\n", stderr: "Couldn't remove the cleanup finalizer of users/" + userName + ": ",
			finalizers: untouched,
		},
		"an object deleted meanwhile": {
			refused: map[string]int32{userPatch: http.StatusNotFound}, stdout: "Cleanup finalizers: 3 removed, 0 failed.\n", finalizers: untouched,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, current)
			cluster.Add(t, accounts()...)
			for request, status := range c.refused {
				cluster.Refuse(request, status)
			}
			result := uninstall(t, kubeconfig, "", yesFlag)
			testutil.Contains(t, stdoutLabel, result.Stdout, uninstalled+c.stdout+teardown)
			testutil.Contains(t, stderrLabel, result.Stderr, c.stderr)
			if c.code == failed {
				testutil.Contains(t, stderrLabel, result.Stderr, "Error: 1 "+left)
			}
			testutil.Equal(t, codeLabel, result.Code, c.code)
			testutil.Equal(t, finalizersLabel, len(finalizers(cluster, users, userName)), c.finalizers)
			testutil.Equal(t, "group", len(finalizers(cluster, groups, groupName)), none)
		})
	}
}

func TestUninstallWarnsWhileAuthStillRuns(t *testing.T) {
	auth := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "telark-auth-service-0", Namespace: namespace, Labels: map[string]string{testutil.LabelComponent: "auth-service"},
	}}
	cluster, kubeconfig := released(t, current)
	cluster.Add(t, append(accounts(), auth)...)
	result := uninstall(t, kubeconfig, "", yesFlag, timeoutFlag, "10ms")
	testutil.Contains(t, stderrLabel, result.Stderr, "auth's pods didn't stop in time (context deadline exceeded): auth can restore the finalizers")
	testutil.Equal(t, stdoutLabel, result.Stdout, uninstallPlan+uninstalled+removedThree)
	testutil.Equal(t, codeLabel, result.Code, passed)
}

func TestUninstallRefusals(t *testing.T) {
	cases := map[string]uninstallCase{
		"an unreadable release": {
			refused: map[string]int32{"GET secrets/": http.StatusInternalServerError}, stderr: "GET secrets/ is refused for this test",
		},
		"a release Helm can't remove": {
			refused: map[string]int32{"DELETE secrets/" + firstRelease: http.StatusInternalServerError}, stderr: "Error: uninstall: ",
			stdout: uninstallPlan,
		},
		"an argument": {args: []string{"extra"}, stderr: `Error: unknown command "extra" for "telark uninstall"` + "\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster, kubeconfig := released(t, current)
			cluster.Add(t, accounts()...)
			for request, status := range c.refused {
				cluster.Refuse(request, status)
			}
			result := uninstall(t, kubeconfig, "", append([]string{yesFlag}, c.args...)...)
			testutil.Contains(t, stderrLabel, result.Stderr, c.stderr)
			testutil.Equal(t, stdoutLabel, result.Stdout, c.stdout)
			testutil.Equal(t, codeLabel, result.Code, failed)
			testutil.Equal(t, finalizersLabel, len(finalizers(cluster, users, userName)), untouched)
		})
	}
}

func TestUninstallWithoutACluster(t *testing.T) {
	t.Setenv("KUBECONFIG", t.TempDir()+"/missing")
	result := testutil.Run(t, uninstallCommand)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n")
	testutil.Equal(t, codeLabel, result.Code, failed)
}
