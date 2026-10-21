package bootstrap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

var (
	lg = constants.GetLogger(constants.LoggerPrefixHelper)

	ErrAction    = errors.New(string(constants.ErrBootstrapActionInvalid))
	ErrNotOpen   = errors.New(string(constants.ErrBootstrapRequestNotOpen))
	ErrNoPasskey = errors.New(string(constants.ErrBootstrapNoPasskey))
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
	if newest := status.Newest(); newest == nil || newest.ID != id || !decidable(newest, action) {
		return nil, ErrNotOpen
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
	return clients.GetBootstrapClient().Cancel(id, telarkconfig.BootstrapReasonDenied), nil
}

// A deny also takes back an approval; a sign-in leaves one alone.
func decidable(request *telarkconfig.BootstrapRequest, action string) bool {
	return request.State == telarkconfig.BootstrapStatePending ||
		request.State == telarkconfig.BootstrapStateApproved && action == constants.BootstrapActionDeny
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
