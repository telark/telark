package bootstrap

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

var (
	lg = constants.GetLogger(constants.LoggerPrefixHelper)

	ErrAction          = errors.New(string(constants.ErrBootstrapActionInvalid))
	ErrNotOpen         = errors.New(string(constants.ErrBootstrapRequestNotOpen))
	ErrNoPasskey       = errors.New(string(constants.ErrBootstrapNoPasskey))
	ErrApproveRecover  = waitError(constants.ErrBootstrapApproveRecover)
	ErrDenyRecovering  = errors.New(string(constants.ErrBootstrapDenyRecovering))
	ErrRecoveryPasskey = errors.New(string(constants.ErrRecoveryPasskey))
)

// The pinned account approves or denies the newest request with a user-verified passkey assertion and no sign-in.
func StartApproval(r *http.Request, id, action string) (*protocol.CredentialAssertion, error) {
	if action != constants.BootstrapActionApprove && action != constants.BootstrapActionDeny {
		return nil, ErrAction
	}
	status, err := pinnedStatus()
	if err != nil {
		return nil, err
	}
	if err := decidableNow(status, id, action); err != nil {
		return nil, err
	}
	user, _, err := pinnedUser(status)
	if err != nil {
		return nil, err
	}
	challenge := make([]byte, constants.BootstrapChallengeRandomBytes)
	if _, err := rand.Read(challenge); err != nil {
		return nil, err
	}
	binding := bindingOf(id, action)
	wa, err := webauthnhelper.GetWebAuthnFor(r)
	if err != nil {
		return nil, err
	}
	options, _, err := wa.BeginLogin(user, webauthn.WithChallenge(append(challenge, binding[:]...)),
		webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrChallengeGenerationFailed), err)
	}
	if err := webauthnhelper.StoreChallengeAt(constants.RedisKeyPrefixBootstrapAssert+id, options.Response.Challenge.String()); err != nil {
		return nil, err
	}
	return options, nil
}

// No session is created and lastLoginAt isn't stamped: an approval isn't a sign-in. The exporter's answer is relayed.
func FinishApproval(r *http.Request, id string, bodyBytes []byte) (*response.GenericResponse, error) {
	var body eps.AssertionBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil ||
		body.Action != constants.BootstrapActionApprove && body.Action != constants.BootstrapActionDeny {
		return nil, ErrAction
	}
	challenge, err := webauthnhelper.ConsumeChallengeAt(constants.RedisKeyPrefixBootstrapAssert+id, bodyBytes)
	if err != nil {
		return nil, err
	}
	if !boundTo(challenge, id, body.Action) {
		return nil, errors.New(string(constants.ErrChallengeNotFound))
	}
	status, err := pinnedStatus()
	if err != nil {
		return nil, err
	}
	user, passkeys, err := pinnedUser(status)
	if err != nil {
		return nil, err
	}
	if _, err := webauthnhelper.VerifyCredential(challenge, user, user.Credentials, passkeys, r, protocol.VerificationRequired); err != nil {
		return nil, err
	}
	if body.Action == constants.BootstrapActionApprove {
		return clients.GetBootstrapClient().Approve(id), nil
	}
	if err := refuseDenyWhileFrozen(status.UserID); err != nil {
		return nil, err
	}
	return clients.GetBootstrapClient().Cancel(id, telarkconfig.BootstrapReasonDenied), nil
}

// The newest request, open to this action: a recovery is never approved, and nothing is denied while break-glass holds
// the account frozen, so a run ends the way it began (its link's passkey would outlive a deny that landed mid-run).
func decidableNow(status *telarkconfig.BootstrapStatus, id, action string) error {
	newest := status.Newest()
	switch {
	case newest == nil || newest.ID != id || !decidable(newest, action):
		return ErrNotOpen
	case action == constants.BootstrapActionApprove && newest.Kind == telarkconfig.BootstrapKindRecover:
		return ErrApproveRecover
	case action == constants.BootstrapActionDeny:
		return refuseDenyWhileFrozen(status.UserID)
	}
	return nil
}

func refuseDenyWhileFrozen(userID string) error {
	err := authhelper.RefuseFrozen(userID, constants.EmptyString)
	if shared.IsError(err, constants.ErrAccountRecovering) {
		return ErrDenyRecovering
	}
	return err
}

// A deny also takes back an approval; a sign-in leaves one alone.
func decidable(request *telarkconfig.BootstrapRequest, action string) bool {
	return request.State == telarkconfig.BootstrapStatePending ||
		request.State == telarkconfig.BootstrapStateApproved && action == constants.BootstrapActionDeny
}

// A passkey a recovery's link saved after the wait's mark, while that recovery is still open, came from a run that
// stopped before it finished: it signs in nothing (the next run removes it), and its refusal cancels nothing.
func RefuseRecoveryPasskey(user *userresource.User, passkeys []*authdata.Passkey, credentialID []byte) error {
	if !user.Bootstrap {
		return nil
	}
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrBootstrapStatusFailed), err))
		return shared.ErrBackendUnavailable
	}
	request := status.Newest()
	if status.UserID != user.ID || request == nil || request.Kind != telarkconfig.BootstrapKindRecover || !request.Open() {
		return nil
	}
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		return err
	}
	if savedSince(passkeys, credentialID, opened.Add(config.BreakGlassWait())) {
		return ErrRecoveryPasskey
	}
	return nil
}

func savedSince(passkeys []*authdata.Passkey, credentialID []byte, mark time.Time) bool {
	for _, passkey := range passkeys {
		if passkey == nil || passkey.CreationTimestamp == nil {
			continue
		}
		id, err := authhelper.DecodeBase64URLWithFallback(passkey.CredentialID)
		created, createdErr := time.Parse(time.RFC3339, *passkey.CreationTimestamp)
		if err == nil && createdErr == nil && bytes.Equal(id, credentialID) && !created.Before(mark) {
			return true
		}
	}
	return false
}

// The authenticator signed this action for this request: another request's assertion, a replay, or a deny
// challenge swapped in Redis for an approve one doesn't match.
func boundTo(challenge *authdata.AuthChallenge, id, action string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(challenge.Challenge)
	if err != nil || len(decoded) != constants.BootstrapChallengeRandomBytes+sha256.Size {
		return false
	}
	binding := bindingOf(id, action)
	return hmac.Equal(decoded[constants.BootstrapChallengeRandomBytes:], binding[:])
}

func bindingOf(id, action string) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.Join(
		[]string{constants.BootstrapChallengeLabel, id, action}, constants.BootstrapChallengeSeparator)))
}

func pinnedStatus() (*telarkconfig.BootstrapStatus, error) {
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrBootstrapStatusFailed), err))
		return nil, shared.ErrBackendUnavailable
	}
	if !status.Pinned() {
		return nil, ErrNotOpen
	}
	return status, nil
}

func pinnedUser(status *telarkconfig.BootstrapStatus) (*webauthnhelper.User, []*authdata.Passkey, error) {
	passkeys, err := clients.GetPasskeyClient().GetAllPasskeysByUser(status.UserID)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedGetPasskeys), err))
		return nil, nil, shared.ErrBackendUnavailable
	}
	credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
	if len(credentials) == constants.InitialCapacity {
		return nil, nil, ErrNoPasskey
	}
	return webauthnhelper.CreateUser(status.UserID, status.Email, status.Email, credentials), passkeys, nil
}

// A sign-in cancels a pending request, so whoever ran break-glass can't wait out an owner who is around. A failed
// cancel never fails the sign-in: the exporter reads lastLoginAt before a request completes.
func CancelOnSignIn(userID string) {
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapSignInNoStatus), err))
		return
	}
	request := status.Newest()
	if status.UserID != userID || request == nil || request.State != telarkconfig.BootstrapStatePending {
		return
	}
	resp := clients.GetBootstrapClient().Cancel(request.ID, telarkconfig.BootstrapReasonSignedIn)
	// Approved meanwhile: a sign-in leaves an approval alone.
	if resp.Status != http.StatusOK && resp.Status != http.StatusConflict {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapSignInCancel), request.ID, resp.Status, resp.Message))
	}
}

// Read when printed, so the text names the configured wait while errors.Is still matches the value.
func (e waitError) Error() string {
	return fmt.Sprintf(string(e), int(config.BreakGlassWait()/time.Hour))
}
