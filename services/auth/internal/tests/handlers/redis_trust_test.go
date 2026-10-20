package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/constants"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	bootstrapID  = "u-boot"
	plantedToken = "planted-token"
	jsonStart    = "{"
)

func signedEntry(t *testing.T, key, value string) string {
	t.Helper()
	entry, err := redishelper.SignEntry(key, value, time.Minute)
	if err != nil {
		t.Fatalf("sign %s: %v", key, err)
	}
	return entry
}

func plant(t *testing.T, key, value string) {
	t.Helper()
	if err := redisServer.Set(key, value); err != nil {
		t.Fatalf("plant %s: %v", key, err)
	}
	t.Cleanup(func() { redisServer.Del(key) })
}

func stored(t *testing.T, key string) string {
	t.Helper()
	value, err := redisServer.Get(key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return value
}

// The entry names want, and auth signed it for its key.
func expectSignedEntry(t *testing.T, key, want string) {
	t.Helper()
	entry, err := redisServer.Get(key)
	if owner, signed := redishelper.VerifyEntry(key, entry); err != nil || !signed || owner != want {
		t.Fatalf("%s = %q (%v), want an entry signed for %q", key, entry, err, want)
	}
}

// The package shares one Redis, so a test counts the ceremonies it adds.
func ceremonies() int {
	count := constants.DefaultInitValue
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, constants.RedisKeyPrefixRegistrationOwner) {
			count++
		}
	}
	return count
}

// The invite directory plus the bootstrap account, which no network path may enroll.
func bootstrapDirectory() *testutil.FakeExporter {
	fake := inviteDirectory()
	fake.Users[bootstrapID] = &userresource.User{ID: bootstrapID, Email: bootstrapEmail, Bootstrap: true,
		Status: userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)}}
	return fake
}

// Break-glass issues the bootstrap account's links, so the test issues one the same way.
func bootstrapLink(t *testing.T) string {
	t.Helper()
	token, _, err := authhelper.IssueInvite(bootstrapID, constants.EmptyString, time.Minute)
	if err != nil {
		t.Fatalf("bootstrap link: %v", err)
	}
	return token
}

// Redis is untrusted: a link entry auth didn't sign for its key, planted for the bootstrap account, swapped onto
// it, copied under another token or stored before the upgrade, opens nothing. It gets the answer an unknown link
// gets, and no ceremony is stored.
func TestUnsignedLinksOpenNothing(t *testing.T) {
	initWebAuthn(t)
	testutil.StubBackend(t, bootstrapDirectory())
	refusal := startWith(unknownToken)
	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{"own link planted for the bootstrap account", func(t *testing.T) string {
			plant(t, constants.RedisKeyPrefixEnrollToken+tokenDigest(plantedToken), bootstrapID)
			return plantedToken
		}},
		{"invite planted for the bootstrap account", func(t *testing.T) string {
			plant(t, constants.RedisKeyPrefixInvite+tokenDigest(plantedToken), bootstrapID)
			return plantedToken
		}},
		{"real link with its owner swapped", func(t *testing.T) string {
			token := issueLink(t, plainTargetID, usersOwner()).Token
			key := constants.RedisKeyPrefixInvite + tokenDigest(token)
			plant(t, key, strings.Replace(stored(t, key), plainTargetID, bootstrapID, constants.DefaultIncrementValue))
			return token
		}},
		{"real link copied under another token", func(t *testing.T) string {
			entry := stored(t, constants.RedisKeyPrefixInvite+tokenDigest(bootstrapLink(t)))
			plant(t, constants.RedisKeyPrefixInvite+tokenDigest(plantedToken), entry)
			return plantedToken
		}},
		{"link stored before the upgrade", func(t *testing.T) string {
			plant(t, constants.RedisKeyPrefixInvite+tokenDigest(plantedToken), plainTargetID)
			return plantedToken
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token := c.token(t)
			before := ceremonies()
			rec := startWith(token)
			testutil.Equal(t, "status", rec.Code, refusal.Code)
			testutil.Equal(t, "answer", rec.Body.String(), refusal.Body.String())
			testutil.Equal(t, "ceremonies", ceremonies(), before)
		})
	}
}

// A finish without a session takes its account from the entries auth signed for that ceremony. An owner planted
// for the bootstrap account (its challenge planted to match), an owner copied from the bootstrap's own ceremony, or
// an enrolled entry copied from another ceremony fails the finish, which then saves no passkey anywhere.
func TestUnsignedCeremoniesSaveNothing(t *testing.T) {
	initWebAuthn(t)
	cases := []struct {
		name  string
		plant func(t *testing.T, ceremony string)
	}{
		{"owner planted for the bootstrap account", func(t *testing.T, ceremony string) {
			plant(t, constants.RedisKeyPrefixRegistrationOwner+ceremony, bootstrapID)
			plant(t, constants.RedisKeyPrefixChallenge+bootstrapID, ceremony)
		}},
		{"owner copied from the bootstrap's ceremony", func(t *testing.T, ceremony string) {
			bootstrapCeremony := testutil.ChallengeDigest(t, challengeOf(t, startWith(bootstrapLink(t))))
			plant(t, constants.RedisKeyPrefixRegistrationOwner+ceremony,
				stored(t, constants.RedisKeyPrefixRegistrationOwner+bootstrapCeremony))
			plant(t, constants.RedisKeyPrefixChallenge+bootstrapID, ceremony)
		}},
		{"enrolled entry copied from another ceremony", func(t *testing.T, ceremony string) {
			token, _, err := authhelper.CreateEnrollToken(plainTargetID)
			if err != nil {
				t.Fatalf("other link: %v", err)
			}
			other := testutil.ChallengeDigest(t, challengeOf(t, startWith(token)))
			plant(t, constants.RedisKeyPrefixEnrolledCeremony+ceremony, stored(t, constants.RedisKeyPrefixEnrolledCeremony+other))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := bootstrapDirectory()
			testutil.StubBackend(t, fake)
			token, _, err := authhelper.CreateEnrollToken(enrolledTargetID)
			if err != nil {
				t.Fatalf("own link: %v", err)
			}
			challenge := challengeOf(t, startWith(token))
			c.plant(t, testutil.ChallengeDigest(t, challenge))

			testutil.Equal(t, "finish", finishWith(t, challenge, webauthnHost).Code, http.StatusBadRequest)
			testutil.Equal(t, "bootstrap passkeys", fake.PasskeyCount(bootstrapID), constants.DefaultInitValue)
			testutil.Equal(t, "own passkeys", fake.PasskeyCount(enrolledTargetID), constants.DefaultIncrementValue)
			testutil.Equal(t, "other passkeys", fake.PasskeyCount(plainTargetID), constants.DefaultInitValue)
		})
	}
}

// A pending self-registration edited in Redis into an Admin with the bootstrap marker fails the finish, and no
// account create reaches the exporter.
func TestEditedPendingUserCreatesNothing(t *testing.T) {
	initWebAuthn(t)
	stub := &proofStub{}
	stubExporter(t, stub)
	awaitSelfRegistration(t, true)
	rec := httptest.NewRecorder()
	passkeyhandler.RegisterStart(rec, jsonReq(`{"email":"`+newEmail+`"}`))
	challenge := challengeOf(t, rec)

	ownerKey := constants.RedisKeyPrefixRegistrationOwner + testutil.ChallengeDigest(t, challenge)
	pendingID, signed := redishelper.VerifyEntry(ownerKey, stored(t, ownerKey))
	testutil.Equal(t, "owner signed", signed, true)
	pendingKey := constants.RedisKeyPrefixPendingUser + pendingID
	entry := stored(t, pendingKey)
	adminRole := constants.BuiltInRoleAdmin
	edited, err := json.Marshal(userresource.User{Username: "new", Email: newEmail, Bootstrap: true, RoleRefs: []*string{&adminRole},
		Status: userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)}})
	if err != nil {
		t.Fatalf("edited user: %v", err)
	}
	signature, _, _ := strings.Cut(entry, jsonStart)
	plant(t, pendingKey, signature+string(edited))

	testutil.Equal(t, "finish", finishWith(t, challenge, webauthnHost).Code, http.StatusBadRequest)
	testutil.Equal(t, "accounts created", len(stub.created), constants.DefaultInitValue)
}
