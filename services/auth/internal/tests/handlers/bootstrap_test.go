package handlers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/rest/base"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/auth/internal/constants"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	bootstraphandler "github.com/telark/telark/services/auth/internal/handlers/bootstrap"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	pendingRequest  = "0123456789abcdef"
	rpName          = "Test"
	otherRequest    = "fedcba9876543210"
	approvalCred    = "approval-credential"
	foreignCred     = "admin-credential"
	approveAction   = "approve"
	denyAction      = "deny"
	flagPresent     = 0x01
	flagVerified    = flagPresent | 0x04
	flagSynced      = flagVerified | 0x08 | 0x10
	getCeremonyType = "webauthn.get"
	approveRoute    = "approve"
	cancelRoute     = "cancel"
	deniedReason    = ` {"reason":"denied"}`
	signedInReason  = ` {"reason":"signed-in"}`
	// A recovery opened this long ago is past the default wait's mark (24 hours); a passkey this old predates it, this young follows it.
	pastTheMark   = 25 * time.Hour
	beforeTheMark = 100 * time.Hour
	afterTheMark  = 30 * time.Minute
)

// The pinned bootstrap account signs with priv, and its newest request, a replacement (a recovery can't be approved),
// is in state.
func pinnedDirectory(t *testing.T, priv *ecdsa.PrivateKey, state string) *testutil.FakeExporter {
	t.Helper()
	fake := bootstrapDirectory()
	fake.Credentials = map[string][]authdata.Passkey{bootstrapID: {{
		UserID:       bootstrapID,
		CredentialID: base64.RawURLEncoding.EncodeToString([]byte(approvalCred)),
		PublicKey:    base64.StdEncoding.EncodeToString(testutil.COSEKeyEC2(t, &priv.PublicKey, coseAlgES256)),
	}}}
	opened := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	request := telarkconfig.BootstrapRequest{ID: pendingRequest, Kind: telarkconfig.BootstrapKindReplace, TargetEmail: newEmail,
		State: state, OpenedAt: opened}
	fake.Bootstrap = telarkconfig.BootstrapStatus{UserID: bootstrapID, Email: bootstrapEmail, PinnedAt: opened,
		Requests: []telarkconfig.BootstrapRequest{request}}
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{
		approveRoute: {Request: request, Eligible: true},
		cancelRoute:  {Request: request},
	}
	return fake
}

// The newest request becomes the pinned account's recovery, opened at opened.
func asRecovery(fake *testutil.FakeExporter, opened time.Time) {
	request := &fake.Bootstrap.Requests[constants.DefaultInitValue]
	request.Kind, request.TargetEmail = telarkconfig.BootstrapKindRecover, constants.EmptyString
	request.OpenedAt = opened.UTC().Format(time.RFC3339)
}

// Frozen as break-glass leaves the account while it waits for the link it printed: the freeze holds that link.
func frozenForLink(t *testing.T, link string) {
	t.Helper()
	t.Cleanup(func() { _ = authhelper.Unfreeze(bootstrapID) })
	if err := authhelper.Freeze(bootstrapID); err != nil {
		t.Fatal(err)
	}
	if link == constants.EmptyString {
		return
	}
	if err := authhelper.FreezeForLink(bootstrapID, link, time.Minute); err != nil {
		t.Fatal(err)
	}
}

// A call the fake exporter recorded: the pending request's route, and the body it was sent.
func requestCall(route, body string) string {
	return strings.Join([]string{"bootstrap", "requests", pendingRequest, route}, "/") + body
}

func approvalPath(endpoint base.Endpoint, id string) string {
	return strings.Replace(router.Pattern(endpoint), idPlaceholder, id, constants.DefaultIncrementValue)
}

// What the approval page posts to start: the options carry the challenge the authenticator signs.
func startApproval(t *testing.T, id, action string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	routed(rec, jsonPost(approvalPath(bootstrapeps.StartAssertion, id), `{"action":"`+action+`"}`))
	if rec.Code != http.StatusOK {
		return rec, constants.EmptyString
	}
	var start bootstraphandler.StartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("start body: %v", err)
	}
	return rec, start.Options.Response.Challenge.String()
}

func finishApproval(t *testing.T, id, body string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	routed(rec, jsonPost(approvalPath(bootstrapeps.FinishAssertion, id), body))
	return rec.Code
}

func jsonPost(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	return r
}

// What a browser posts once the authenticator holding priv signed challenge, with the extra fields beside it.
func assertionBody(t *testing.T, priv *ecdsa.PrivateKey, flags byte, challenge string, extra map[string]any) string {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(webauthnHost))
	authData := binary.BigEndian.AppendUint32(append(append([]byte{}, rpIDHash[:]...), flags), constants.DefaultIncrementValue)
	clientData, err := json.Marshal(map[string]any{"type": getCeremonyType, "challenge": challenge, "origin": "http://" + webauthnHost})
	if err != nil {
		t.Fatal(err)
	}
	clientDataHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(slices.Concat(authData, clientDataHash[:]))
	signature, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	body := map[string]any{"id": enc([]byte(approvalCred)), "rawId": enc([]byte(approvalCred)), "type": "public-key",
		"response": map[string]any{"authenticatorData": enc(authData), "clientDataJSON": enc(clientData), "signature": enc(signature)}}
	for key, value := range extra {
		body[key] = value
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func approvalBody(t *testing.T, priv *ecdsa.PrivateKey, flags byte, challenge, action string) string {
	t.Helper()
	return assertionBody(t, priv, flags, challenge, map[string]any{"action": action})
}

// Starts a ceremony for action on the pending request and finishes it with an assertion over the challenge
// it issued, signed with flags and posted for action as the body names it.
func decide(t *testing.T, priv *ecdsa.PrivateKey, flags byte, started, posted string) int {
	t.Helper()
	_, challenge := startApproval(t, pendingRequest, started)
	return finishApproval(t, pendingRequest, approvalBody(t, priv, flags, challenge, posted))
}

// Each approval or deny is a new user-verified signature by the pinned account over this request and this action. A
// finish without a start, a replay, a missing user-verified flag, another request's assertion, a challenge planted
// in Redis without the account's key, or an action flipped in the body moves nothing; none creates a session.
func TestApprovalNeedsAFreshAssertion(t *testing.T) {
	initWebAuthn(t)
	priv := testutil.NewP256Key(t)
	assertKey := constants.RedisKeyPrefixBootstrapAssert + pendingRequest
	cases := []struct {
		name  string
		run   func(t *testing.T) []int
		want  []int
		calls []string
	}{
		{"approved", func(t *testing.T) []int { return []int{decide(t, priv, flagVerified, approveAction, approveAction)} },
			[]int{http.StatusOK}, []string{requestCall(approveRoute, constants.EmptyString)}},
		{"denied", func(t *testing.T) []int { return []int{decide(t, priv, flagVerified, denyAction, denyAction)} },
			[]int{http.StatusOK}, []string{requestCall(cancelRoute, deniedReason)}},
		{"finish without a start", func(t *testing.T) []int {
			challenge := base64.RawURLEncoding.EncodeToString(make([]byte, constants.BootstrapChallengeRandomBytes+sha256.Size))
			return []int{finishApproval(t, pendingRequest, approvalBody(t, priv, flagVerified, challenge, approveAction))}
		}, []int{http.StatusUnauthorized}, nil},
		{"replayed assertion", func(t *testing.T) []int {
			_, challenge := startApproval(t, pendingRequest, approveAction)
			body := approvalBody(t, priv, flagVerified, challenge, approveAction)
			return []int{finishApproval(t, pendingRequest, body), finishApproval(t, pendingRequest, body)}
		}, []int{http.StatusOK, http.StatusUnauthorized}, []string{requestCall(approveRoute, constants.EmptyString)}},
		{"no user verification", func(t *testing.T) []int { return []int{decide(t, priv, flagPresent, approveAction, approveAction)} },
			[]int{http.StatusUnauthorized}, nil},
		{"another request's assertion", func(t *testing.T) []int {
			_, challenge := startApproval(t, pendingRequest, approveAction)
			return []int{finishApproval(t, otherRequest, approvalBody(t, priv, flagVerified, challenge, approveAction))}
		}, []int{http.StatusUnauthorized}, nil},
		{"approve challenge planted in Redis", func(t *testing.T) []int {
			challenge := base64.RawURLEncoding.EncodeToString(slices.Concat(
				make([]byte, constants.BootstrapChallengeRandomBytes), bindingOf(pendingRequest, approveAction)))
			plant(t, assertKey+constants.ColonSeparator+testutil.ChallengeDigest(t, challenge), constants.RedisChallengeEntry)
			return []int{finishApproval(t, pendingRequest, approvalBody(t, testutil.NewP256Key(t), flagVerified, challenge, approveAction))}
		}, []int{http.StatusUnauthorized}, nil},
		{"action flipped in the body", func(t *testing.T) []int { return []int{decide(t, priv, flagVerified, denyAction, approveAction)} },
			[]int{http.StatusUnauthorized}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
			testutil.StubBackend(t, fake)
			redisServer.Del(assertKey)
			if got := c.run(t); !slices.Equal(got, c.want) {
				t.Fatalf("answers = %v, want %v", got, c.want)
			}
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), strings.Join(c.calls, ","))
			testutil.Equal(t, "sessions", fake.SessionsIssued(), constants.DefaultInitValue)
			testutil.Equal(t, "lastLoginAt stamped", fake.User(bootstrapID).Status.LastLoginAt != nil, false)
		})
	}
}

// A synced passkey (backup eligible, backed up) approves like any other, and a forged assertion claiming those flags
// is refused: go-webauthn checks the signature and user verification before the backup-flag check, the one error
// auth's fallback (HandleBackupFlagError) lets through.
func TestSyncedPasskeyApproves(t *testing.T) {
	initWebAuthn(t)
	priv := testutil.NewP256Key(t)
	cases := []struct {
		name   string
		signer *ecdsa.PrivateKey
		want   int
	}{
		{"its own signature", priv, http.StatusOK},
		{"another key's signature", testutil.NewP256Key(t), http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
			stored := &fake.Credentials[bootstrapID][0]
			stored.BackupEligible, stored.BackupState = true, true
			testutil.StubBackend(t, fake)
			redisServer.Del(constants.RedisKeyPrefixBootstrapAssert + pendingRequest)
			_, challenge := startApproval(t, pendingRequest, approveAction)
			testutil.Equal(t, "answer", finishApproval(t, pendingRequest, approvalBody(t, c.signer, flagSynced, challenge, approveAction)), c.want)
		})
	}
}

// The suffix auth binds into an approval challenge: SHA-256 of the label, the request id and the action.
func bindingOf(id, action string) []byte {
	sum := sha256.Sum256([]byte(strings.Join([]string{constants.BootstrapChallengeLabel, id, action},
		constants.BootstrapChallengeSeparator)))
	return sum[:]
}

// Each ceremony has its own key: another start for the same account or request, or a stray finish naming it,
// neither replaces nor spends the owner's ceremony, so nobody without a credential can keep the bootstrap account
// from signing in (which cancels a request) or from denying one.
func TestOthersCantSpoilACeremony(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	stray := func(t *testing.T, extra map[string]any) string {
		challenge := base64.RawURLEncoding.EncodeToString(make([]byte, constants.BootstrapChallengeRandomBytes))
		return assertionBody(t, testutil.NewP256Key(t), flagVerified, challenge, extra)
	}
	email := map[string]any{"email": bootstrapEmail}
	cases := []struct {
		name string
		run  func(t *testing.T) []int
	}{
		{"sign-in", func(t *testing.T) []int {
			challenge := signInChallenge(t)
			signInChallenge(t)
			junk := httptest.NewRecorder()
			authhandler.LoginFinish(junk, jsonReq(stray(t, email)))
			rec := httptest.NewRecorder()
			authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, challenge, email)))
			return []int{junk.Code, rec.Code}
		}},
		{"deny", func(t *testing.T) []int {
			_, challenge := startApproval(t, pendingRequest, denyAction)
			startApproval(t, pendingRequest, approveAction)
			junk := finishApproval(t, pendingRequest, stray(t, map[string]any{"action": denyAction}))
			return []int{junk, finishApproval(t, pendingRequest, approvalBody(t, priv, flagVerified, challenge, denyAction))}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.StubBackend(t, pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending))
			if got := c.run(t); !slices.Equal(got, []int{http.StatusUnauthorized, http.StatusOK}) {
				t.Fatalf("stray finish, then the owner's = %v, want [401 200]", got)
			}
		})
	}
}

// While break-glass recovers an account, it starts and finishes no sign-in or registration and mints no link,
// even with a ceremony opened before; the same account works again once thawed.
func TestFrozenAccountRefusesCeremonies(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	testutil.StubBackend(t, pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending))
	opened := signInChallenge(t)
	if err := authhelper.Freeze(bootstrapID); err != nil {
		t.Fatal(err)
	}
	start := httptest.NewRecorder()
	authhandler.LoginStart(start, jsonReq(`{"email":"`+bootstrapEmail+`"}`))
	finish := httptest.NewRecorder()
	authhandler.LoginFinish(finish, jsonReq(assertionBody(t, priv, flagVerified, opened, map[string]any{"email": bootstrapEmail})))
	_, _, linkErr := authhelper.CreateEnrollToken(bootstrapID)
	testutil.Equal(t, "frozen login start", start.Code, http.StatusConflict)
	testutil.Equal(t, "frozen login finish", finish.Code, http.StatusConflict)
	testutil.Equal(t, "link refused", linkErr != nil && linkErr.Error() == string(constants.ErrAccountRecovering), true)

	if err := authhelper.Unfreeze(bootstrapID); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t), map[string]any{"email": bootstrapEmail})))
	testutil.Equal(t, "thawed sign-in", rec.Code, http.StatusOK)
}

// Only the newest request, pending (or approved, for a deny), with a pinned account that has a passkey, can be decided,
// and a recovery only denied. A refused start stores no challenge.
func TestApprovalStartRefuses(t *testing.T) {
	initWebAuthn(t)
	priv := testutil.NewP256Key(t)
	pending := telarkconfig.BootstrapStatePending
	cases := []struct {
		name     string
		state    string
		id       string
		action   string
		bare     bool
		recovery bool
		want     int
	}{
		{"completed request", telarkconfig.BootstrapStateCompleted, pendingRequest, approveAction, false, false, http.StatusConflict},
		{"approved request, approve again", telarkconfig.BootstrapStateApproved, pendingRequest, approveAction, false, false, http.StatusConflict},
		{"approved request, deny", telarkconfig.BootstrapStateApproved, pendingRequest, denyAction, false, false, http.StatusOK},
		{"not the newest request", pending, otherRequest, approveAction, false, false, http.StatusConflict},
		{"unknown action", pending, pendingRequest, "promote", false, false, http.StatusBadRequest},
		{"pinned account without a passkey", pending, pendingRequest, approveAction, true, false, http.StatusConflict},
		{"recovery, approve", pending, pendingRequest, approveAction, false, true, http.StatusConflict},
		{"recovery, deny", pending, pendingRequest, denyAction, false, true, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, c.state)
			if c.bare {
				fake.Credentials = nil
			}
			if c.recovery {
				asRecovery(fake, time.Now().Add(-time.Hour))
			}
			testutil.StubBackend(t, fake)
			challenges := approvalChallenges()
			rec, _ := startApproval(t, c.id, c.action)
			testutil.Equal(t, "status", rec.Code, c.want)
			testutil.Equal(t, "challenge stored", approvalChallenges() > challenges, c.want == http.StatusOK)
			if c.recovery && c.action == approveAction {
				testutil.Equal(t, "states the wait", strings.Contains(rec.Body.String(), "completes 24 hours after it opened"), true)
			}
		})
	}
}

// The package shares one Redis, so a test counts the approval ceremonies it adds.
func approvalChallenges() int {
	count := constants.DefaultInitValue
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, constants.RedisKeyPrefixBootstrapAssert) {
			count++
		}
	}
	return count
}

// While break-glass waits for the link it printed, the freeze holds that link: the registration it opens starts and
// saves a passkey, where a sign-in, the account's own earlier link and a deny are refused (409).
func TestRecoveryLinkRegistersWhileFrozen(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	fake := pinnedDirectory(t, testutil.NewP256Key(t), telarkconfig.BootstrapStatePending)
	asRecovery(fake, time.Now().Add(-pastTheMark))
	testutil.StubBackend(t, fake)
	ownLink, _, err := authhelper.CreateEnrollToken(bootstrapID)
	if err != nil {
		t.Fatal(err)
	}
	link := bootstrapLink(t)
	frozenForLink(t, link)

	signIn := httptest.NewRecorder()
	authhandler.LoginStart(signIn, jsonReq(`{"email":"`+bootstrapEmail+`"}`))
	testutil.Equal(t, "sign-in start", signIn.Code, http.StatusConflict)
	testutil.Equal(t, "the account's own link", startWith(ownLink).Code, http.StatusConflict)
	deny, _ := startApproval(t, pendingRequest, denyAction)
	testutil.Equal(t, "deny start", deny.Code, http.StatusConflict)
	testutil.Equal(t, "the recovery's link", finishWith(t, challengeOf(t, startWith(link)), webauthnHost).Code, http.StatusCreated)
	testutil.Equal(t, "passkeys saved", fake.PasskeyCount(bootstrapID), constants.DefaultIncrementValue)
	testutil.Equal(t, "exporter calls", len(fake.RequestCalls()), constants.DefaultInitValue)
}

// A deny started before break-glass froze the account and finished while it is frozen is refused too, and reaches no
// exporter: the run in progress ends the way it began.
func TestDenyWaitsForTheRun(t *testing.T) {
	initWebAuthn(t)
	priv := testutil.NewP256Key(t)
	fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
	asRecovery(fake, time.Now().Add(-time.Hour))
	testutil.StubBackend(t, fake)
	_, challenge := startApproval(t, pendingRequest, denyAction)
	frozenForLink(t, constants.EmptyString)
	deny := finishApproval(t, pendingRequest, approvalBody(t, priv, flagVerified, challenge, denyAction))
	testutil.Equal(t, "deny finish", deny, http.StatusConflict)
	testutil.Equal(t, "exporter calls", len(fake.RequestCalls()), constants.DefaultInitValue)
}

// A passkey a recovery's link saved after the wait's mark, while that recovery is still open, came from a run that
// stopped before it finished: the bootstrap account's sign-in with it is refused (409) before any session, and cancels
// nothing. One saved before the request signs in, and cancels the request.
func TestUnfinishedRecoveryPasskeySignsNothingIn(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	cases := []struct {
		name     string
		created  time.Time
		want     int
		sessions int
		calls    []string
	}{
		{"passkey saved before the request", time.Now().Add(-beforeTheMark), http.StatusOK, constants.DefaultIncrementValue,
			[]string{requestCall(cancelRoute, signedInReason)}},
		{"passkey a link saved after the mark", time.Now().Add(-afterTheMark), http.StatusConflict, constants.DefaultInitValue, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
			asRecovery(fake, time.Now().Add(-pastTheMark))
			created := c.created.UTC().Format(time.RFC3339)
			fake.Credentials[bootstrapID][constants.DefaultInitValue].CreationTimestamp = &created
			testutil.StubBackend(t, fake)
			rec := httptest.NewRecorder()
			authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t),
				map[string]any{"email": bootstrapEmail})))
			testutil.Equal(t, "login", rec.Code, c.want)
			testutil.Equal(t, "sessions", fake.SessionsIssued(), c.sessions)
			testutil.Equal(t, "lastLoginAt stamped", fake.User(bootstrapID).Status.LastLoginAt != nil, c.want == http.StatusOK)
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), strings.Join(c.calls, ","))
		})
	}
}

// A validly signed assertion by another account's passkey over the request's challenge, carrying handle as its user
// handle when set.
func foreignAssertion(t *testing.T, priv *ecdsa.PrivateKey, handle []byte, challenge string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(approvalBody(t, priv, flagVerified, challenge, approveAction)), &body); err != nil {
		t.Fatal(err)
	}
	response, isMap := body["response"].(map[string]any)
	if !isMap {
		t.Fatal("assertion without a response")
	}
	id := base64.RawURLEncoding.EncodeToString([]byte(foreignCred))
	body["id"], body["rawId"] = id, id
	if handle != nil {
		response["userHandle"] = base64.RawURLEncoding.EncodeToString(handle)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// Only the pinned account's own passkeys decide its requests: another account's passkey, validly signed over the
// request's challenge, is refused (401) whatever user handle it carries, and reaches no exporter.
func TestAnotherAccountsPasskeyDecidesNothing(t *testing.T) {
	initWebAuthn(t)
	owner, other := testutil.NewP256Key(t), testutil.NewP256Key(t)
	cases := []struct {
		name   string
		handle []byte
	}{
		{"no user handle", nil},
		{"its own user handle", []byte(adminCallID)},
		{"the pinned account's user handle", []byte(bootstrapID)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, owner, telarkconfig.BootstrapStatePending)
			fake.Credentials[adminCallID] = []authdata.Passkey{{
				UserID:       adminCallID,
				CredentialID: base64.RawURLEncoding.EncodeToString([]byte(foreignCred)),
				PublicKey:    base64.StdEncoding.EncodeToString(testutil.COSEKeyEC2(t, &other.PublicKey, coseAlgES256)),
			}}
			testutil.StubBackend(t, fake)
			_, challenge := startApproval(t, pendingRequest, approveAction)
			testutil.Equal(t, "answer", finishApproval(t, pendingRequest, foreignAssertion(t, other, c.handle, challenge)),
				http.StatusUnauthorized)
			testutil.Equal(t, "exporter calls", len(fake.RequestCalls()), constants.DefaultInitValue)
			testutil.Equal(t, "sessions", fake.SessionsIssued(), constants.DefaultInitValue)
		})
	}
}

// What login/start issues for the bootstrap account to sign.
func signInChallenge(t *testing.T) string {
	t.Helper()
	return challengeFor(t, bootstrapEmail)
}

func challengeFor(t *testing.T, email string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	authhandler.LoginStart(rec, jsonReq(`{"email":"`+email+`"}`))
	testutil.Equal(t, "login start", rec.Code, http.StatusOK)
	var start authhandler.LoginStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	return start.Options.Response.Challenge.String()
}

// A sign-in by the bootstrap account cancels its pending request and leaves an approved one alone. A cancel the
// exporter refuses never fails the sign-in: the exporter reads lastLoginAt before a request completes.
func TestSignInCancelsAPendingRequest(t *testing.T) {
	initWebAuthn(t)
	// A session is issued from the service config, which needs a relying-party name.
	t.Setenv(constants.EnvRPName, rpName)
	priv := testutil.NewP256Key(t)
	canceled := []string{requestCall(cancelRoute, signedInReason)}
	cases := []struct {
		name    string
		state   string
		refuses bool
		calls   []string
	}{
		{"pending", telarkconfig.BootstrapStatePending, false, canceled},
		{"approved", telarkconfig.BootstrapStateApproved, false, nil},
		{"cancel refused", telarkconfig.BootstrapStatePending, true, canceled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := pinnedDirectory(t, priv, c.state)
			if c.refuses {
				delete(fake.RequestAnswers, cancelRoute)
			}
			testutil.StubBackend(t, fake)
			rec := httptest.NewRecorder()
			authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t),
				map[string]any{"email": bootstrapEmail})))
			testutil.Equal(t, "login", rec.Code, http.StatusOK)
			testutil.Equal(t, "sessions", fake.SessionsIssued(), constants.DefaultIncrementValue)
			// Stamped before the answer: the wait's backstop reads this stamp.
			testutil.Equal(t, "lastLoginAt stamped", fake.User(bootstrapID).Status.LastLoginAt != nil, true)
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), strings.Join(c.calls, ","))
		})
	}
}

// Neither the challenge nor any encoding of its bytes is in a Redis key or value, and its digest is.
func expectOnlyDigest(t *testing.T, challenge string) {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil || challenge == constants.EmptyString {
		t.Fatalf("challenge %q: %v", challenge, err)
	}
	digest := testutil.ChallengeDigest(t, challenge)
	held := false
	for _, key := range redisServer.Keys() {
		value, _ := redisServer.Get(key)
		for _, form := range []string{challenge, string(raw), base64.StdEncoding.EncodeToString(raw)} {
			if strings.Contains(key, form) || strings.Contains(value, form) {
				t.Fatalf("the challenge is stored in Redis under %q", key)
			}
		}
		held = held || value == digest || strings.HasSuffix(key, digest)
	}
	testutil.Equal(t, "digest stored", held, true)
}

// Redis holds a ceremony's challenge only as its digest, in keys and values alike: whoever reads it during a link,
// signed-in registration, sign-in or approval ceremony can't answer the challenge before the browser does.
func TestRedisHoldsNoChallenge(t *testing.T) {
	initWebAuthn(t)
	priv := testutil.NewP256Key(t)
	cases := []struct {
		name  string
		start func(t *testing.T) string
	}{
		{"link ceremony", func(t *testing.T) string {
			testutil.StubBackend(t, inviteDirectory())
			token, _, err := authhelper.CreateEnrollToken(enrolledTargetID)
			if err != nil {
				t.Fatal(err)
			}
			return challengeOf(t, startWith(token))
		}},
		{"session registration", func(t *testing.T) string {
			stubExporter(t, registerStub{session: true})
			r := jsonReq(constants.EmptyString)
			r.Header.Set(constants.HeaderSessionToken, "token")
			rec := httptest.NewRecorder()
			passkeyhandler.RegisterStart(rec, r)
			return challengeOf(t, rec)
		}},
		{"sign-in", func(t *testing.T) string {
			testutil.StubBackend(t, pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending))
			return signInChallenge(t)
		}},
		{"approval", func(t *testing.T) string {
			testutil.StubBackend(t, pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending))
			_, challenge := startApproval(t, pendingRequest, approveAction)
			return challenge
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectOnlyDigest(t, c.start(t))
		})
	}
}

// Sets the freeze, as break-glass does, when a request on the account's record carries mark (in its body, if any): a
// request that passed its first freeze checks reaches its next write only once the account is frozen.
type freezeMidway struct {
	inner  http.Handler
	method string
	userID string
	mark   string
}

func (f freezeMidway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	if r.Method == f.method && strings.HasSuffix(r.URL.Path, "/users/"+f.userID) && strings.Contains(string(body), f.mark) {
		_ = authhelper.Freeze(f.userID)
	}
	f.inner.ServeHTTP(w, r)
}

// Freezes each account the stub created, as break-glass would the moment it exists.
type freezeOnCreate struct{ *proofStub }

func (f freezeOnCreate) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := f.proofStub.RoundTrip(r)
	for _, user := range f.created {
		if freezeErr := authhelper.Freeze(user.ID); freezeErr != nil {
			return nil, freezeErr
		}
	}
	return resp, err
}

// A request that began before break-glass froze the account checks the freeze again right before each write, so it
// writes no passkey and no session once the account is frozen: nothing it sends can land after the bound break-glass
// waits out.
func TestFrozenAccountGetsNoLateWrite(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	t.Cleanup(func() { redisServer.FlushAll() })
	t.Run("registration", func(t *testing.T) {
		fake := inviteDirectory()
		testutil.StubBackend(t, freezeMidway{inner: fake, method: http.MethodPatch, userID: plainTargetID, mark: "identities"})
		challenge := challengeOf(t, startWith(issueLink(t, plainTargetID, usersOwner()).Token))
		testutil.Equal(t, "finish", finishWith(t, challenge, webauthnHost).Code, http.StatusConflict)
		testutil.Equal(t, "passkeys", fake.PasskeyCount(plainTargetID), constants.DefaultInitValue)
	})
	t.Run("self-registration", func(t *testing.T) {
		stub := &proofStub{}
		stubExporter(t, freezeOnCreate{stub})
		awaitSelfRegistration(t, true)
		rec := httptest.NewRecorder()
		passkeyhandler.RegisterStart(rec, jsonReq(`{"email":"`+newEmail+`"}`))
		testutil.Equal(t, "finish", finishWith(t, challengeOf(t, rec), webauthnHost).Code, http.StatusConflict)
		testutil.Equal(t, "accounts deleted", stub.deleted, constants.DefaultIncrementValue)
	})
	t.Run("late session", func(t *testing.T) {
		priv := testutil.NewP256Key(t)
		fake := pinnedDirectory(t, priv, telarkconfig.BootstrapStatePending)
		testutil.StubBackend(t, freezeMidway{inner: fake, method: http.MethodGet, userID: bootstrapID})
		rec := httptest.NewRecorder()
		authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t),
			map[string]any{"email": bootstrapEmail})))
		testutil.Equal(t, "login", rec.Code, http.StatusConflict)
		testutil.Equal(t, "sessions", fake.SessionsIssued(), constants.DefaultInitValue)
	})
}
