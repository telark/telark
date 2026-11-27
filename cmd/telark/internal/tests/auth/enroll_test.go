package auth

import (
	"crypto/rand"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"golang.org/x/crypto/nacl/box"
)

const (
	enrollLink       = "https://telark.example.com/register#enroll=token&email=test%40example.com"
	linkExpiresAt    = "2026-11-23T12:10:00Z"
	recovering       = "ending the sessions of test@example.com and revoking what was saved after the wait, before the link"
	linkReady        = "register a passkey for test@example.com with the link, in a browser that is not signed in to Telark"
	enrolled         = "test@example.com registered a passkey with the link"
	waitingReason    = "request req-1 is open: recover test@example.com. Run telark auth enroll again after completesAt"
	placeholderURL   = "<dashboard-url>/auth/request#request=req-1&kind=recover"
	completesAt      = "2026-11-24T12:00:00Z"
	created          = "Created AuthRequest auth-request-00001 as ops-admin. Waiting for auth to run it\n"
	refusedWhoAmI    = "POST selfsubjectreviews.authentication.k8s.io/"
	refusedCreate    = "POST authrequests.telark.io/"
	refusedDiscovery = "GET .telark.io/"
)

// The command creates the object as the caller, follows it through each phase auth reports, and ends once the
// passkey is registered. Without watch lists, client-go lists first and then watches.
func TestEnrollPrintsTheLinkAndWaitsForThePasskey(t *testing.T) {
	for name, noWatchList := range map[string]bool{"watch list": false, "list, then watch": true} {
		t.Run(name, func(t *testing.T) {
			fake := testutil.StartCluster(t, testutil.Cluster{NoWatchList: noWatchList})
			done, stream := enroll(t, fake)
			name := fake.Created(t, requests)
			fake.Watching(t, requests)
			fake.SetStatus(t, requests, name, map[string]any{keyPhase: "Pending"})
			fake.SetStatus(t, requests, name, map[string]any{keyPhase: "Recovering", keyReason: recovering})
			stream.Await(t, recovering)
			// A status write that keeps the phase prints nothing more.
			fake.SetStatus(t, requests, name, map[string]any{keyPhase: "Recovering", keyReason: recovering})
			fake.SetStatus(t, requests, name, map[string]any{
				keyPhase: "LinkReady", keyReason: linkReady, keySealedLink: seal(t, fake, name, enrollLink), keyLinkExpires: linkExpiresAt,
			})
			stream.Await(t, constants.MsgWaitingPasskey)
			fake.SetStatus(t, requests, name, map[string]any{keyPhase: "Completed", keyReason: enrolled})
			result := testutil.Wait(t, done)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
			testutil.Equal(t, stdoutLabel, stream.String(), created+
				"Recovering: "+recovering+"\n"+
				"LinkReady: "+linkReady+"\n"+
				"  link:          "+enrollLink+"\n"+
				"  linkExpiresAt: "+linkExpiresAt+"\n"+
				"Waiting for the passkey\n"+
				"✔ Completed: "+enrolled+"\n")
			requested := field(t, fake, name, keySpec)
			testutil.Equal(t, keyEmail, requested[keyEmail], any(adminEmail))
			testutil.Equal(t, keyRequestedBy, requested[keyRequestedBy], any(testutil.DefaultUser))
			testutil.Equal(t, keyGenerate, field(t, fake, name, keyMetadata)[keyGenerate], any(constants.AuthRequestPrefix))
		})
	}
}

// The object rests at WaitingForApproval, so the command prints the page and the time, and ends.
func TestEnrollEndsWhileTheRequestWaits(t *testing.T) {
	fake := testutil.StartCluster(t, testutil.Cluster{User: "release_lead"})
	done, stream := enroll(t, fake)
	name := fake.Created(t, requests)
	fake.SetStatus(t, requests, name, map[string]any{
		keyPhase: "WaitingForApproval", keyReason: waitingReason, keyApproval: placeholderURL, keyCompletes: completesAt,
	})
	result := testutil.Wait(t, done)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeSuccess)
	testutil.Equal(t, stdoutLabel, stream.String(), "Created AuthRequest auth-request-00001 as release_lead. Waiting for auth to run it\n"+
		"‼ WaitingForApproval: "+waitingReason+"\n"+
		"  approvalUrl:   "+placeholderURL+"\n"+
		"  completesAt:   "+completesAt+"\n"+
		"  Replace <dashboard-url> with the address you open the dashboard at\n")
	testutil.Equal(t, keyRequestedBy, field(t, fake, name, keySpec)[keyRequestedBy], any("release_lead"))
}

func TestEnrollFailsWhenTheRequestEnds(t *testing.T) {
	for _, phase := range []string{"Denied", "Canceled", "Expired", "Failed"} {
		t.Run(phase, func(t *testing.T) {
			fake := testutil.StartCluster(t, testutil.Cluster{})
			done, _ := enroll(t, fake)
			name := fake.Created(t, requests)
			fake.SetStatus(t, requests, name, map[string]any{keyPhase: phase, keyReason: "the reason auth gave"})
			result := testutil.Wait(t, done)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Equal(t, stderrLabel, result.Stderr, "Error: AuthRequest auth-request-00001 ended "+phase+": the reason auth gave\n")
		})
	}
}

func TestEnrollFailsWhenTheRequestIsDeleted(t *testing.T) {
	fake := testutil.StartCluster(t, testutil.Cluster{})
	done, stream := enroll(t, fake)
	name := fake.Created(t, requests)
	fake.SetStatus(t, requests, name, map[string]any{keyPhase: "Recovering", keyReason: recovering})
	stream.Await(t, recovering)
	fake.Remove(t, requests, name)
	result := testutil.Wait(t, done)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: AuthRequest auth-request-00001 was deleted\n")
}

// Only the key the command made opens the link, so a link sealed to any other key is refused, not printed.
func TestEnrollRefusesALinkSealedToAnotherKey(t *testing.T) {
	fake := testutil.StartCluster(t, testutil.Cluster{})
	done, stream := enroll(t, fake)
	name := fake.Created(t, requests)
	other, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fake.SetStatus(t, requests, name, map[string]any{keyPhase: "LinkReady", keyReason: linkReady, keySealedLink: sealTo(t, other, enrollLink)})
	result := testutil.Wait(t, done)
	testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
	testutil.Equal(t, stderrLabel, result.Stderr, "Error: AuthRequest auth-request-00001: the link isn't sealed to this command's key\n")
	if strings.Contains(stream.String(), enrollLink) {
		t.Errorf("stdout shows the link: %q", stream.String())
	}
}

func TestEnrollFailsBeforeTheRequest(t *testing.T) {
	cases := []struct {
		name    string
		cluster testutil.Cluster
		refuse  string
		status  int32
		want    string
	}{
		{
			name:    "a release older than 0.3.0",
			cluster: testutil.Cluster{Missing: []string{authRequestsAt}},
			want: "Error: this Telark serves no AuthRequest objects, so it is older than 0.3.0: run " +
				"`kubectl exec -n telark deploy/telark-auth-service -- ./main break-glass --email test@example.com --enroll` instead\n",
		},
		{
			name:   "no telark.io API",
			refuse: refusedDiscovery,
			status: http.StatusNotFound,
			want:   "Error: this Telark serves no AuthRequest objects, so it is older than 0.3.0: run ",
		},
		{name: "who am I refused", refuse: refusedWhoAmI, status: http.StatusForbidden, want: "Error: read your Kubernetes username: "},
		{name: "create refused", refuse: refusedCreate, status: http.StatusForbidden, want: "Error: create the AuthRequest: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := testutil.StartCluster(t, c.cluster)
			if c.refuse != "" {
				fake.Refuse(c.refuse, c.status)
			}
			result := testutil.Run(t, authCommand, enrollCommand, emailFlag, adminEmail, kubeconfigFlag, kubeconfig(t, fake))
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Contains(t, stderrLabel, result.Stderr, c.want)
			if writes := fake.Writes(); len(writes) != 0 {
				t.Errorf("writes: got %v, want none", writes)
			}
		})
	}
}

func TestEnrollNeedsACluster(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"no cluster": {
			args: []string{emailFlag, adminEmail},
			want: "Error: no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG\n",
		},
		"a kubeconfig that doesn't exist": {
			args: []string{emailFlag, adminEmail, kubeconfigFlag, missingConfig},
			want: "Error: stat /nonexistent/kubeconfig: no such file or directory\n",
		},
		"the API is forbidden": {
			args: []string{
				emailFlag, adminEmail, kubeconfigFlag,
				testutil.Kubeconfig(t, contextName, map[string]string{contextName: testutil.ForbiddenAPIServer(t)}),
			},
			want: "Error: read the telark.io API: ",
		},
		"no email": {args: []string{kubeconfigFlag, missingConfig}, want: `Error: required flag(s) "email" not set`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(kubeconfigEnv, missingConfig)
			result := testutil.Run(t, append([]string{authCommand, enrollCommand}, c.args...)...)
			testutil.Equal(t, codeLabel, result.Code, constants.ExitCodeFailure)
			testutil.Contains(t, stderrLabel, result.Stderr, c.want)
		})
	}
}
