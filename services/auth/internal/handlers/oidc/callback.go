package oidc

import (
	"errors"
	"fmt"
	"net/http"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var lg = constants.GetLogger(constants.LoggerPrefixOIDC)

func GoogleCallback(w http.ResponseWriter, r *http.Request) {
	oidcCfg, err := oidchelper.LoadConfig()
	if err != nil {
		shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
		return
	}

	if !isOIDCConfigured(w, oidcCfg) {
		return
	}

	var req CallbackRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	claims, err := oidchelper.ValidateGoogleIDToken(req.IDToken, oidcCfg)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
		return
	}

	if !isNonceAccepted(w, claims.Nonce) {
		return
	}

	if !isEmailVerified(w, claims) {
		return
	}

	user, ok := resolveOIDCUser(w, claims)
	if !ok {
		return
	}

	sessionToken, err := authhelper.CreateUserSession(user.ID, &req.DeviceMetadata)
	if err != nil {
		sendCallbackError(w, fmt.Errorf(string(constants.ErrOIDCCreateSessionFailed), err), shared.GetStatusCodeForSessionError(err))
		return
	}

	capturedUserID := user.ID
	authhelper.Dispatch(func() {
		authhelper.UpdateUserLastLogin(capturedUserID)
	})

	lg.Info(fmt.Sprintf(string(constants.LogOIDCLoginAccepted), shared.IdentityHash(claims.Subject)))
	shared.SendSuccessResponse(w, string(constants.SuccessOIDCLoginCompleted), CallbackResponse{
		SessionToken: sessionToken,
		Email:        claims.Email,
		User:         user,
	})
}

func isOIDCConfigured(w http.ResponseWriter, oidc telarkconfigresource.OIDCConfig) bool {
	if oidchelper.Usable(oidc) {
		return true
	}
	shared.HandleError(w, errors.New(string(constants.ErrOIDCNotConfigured)),
		http.StatusServiceUnavailable, string(constants.ErrOIDCNotConfigured))
	return false
}

// A missing or spent nonce is the caller's; Redis failing to answer is logged.
func isNonceAccepted(w http.ResponseWriter, nonce string) bool {
	err := oidchelper.VerifyAndConsumeNonce(nonce)
	switch {
	case err == nil:
		return true
	case shared.IsError(err, constants.ErrOIDCNonceMissing), shared.IsError(err, constants.ErrOIDCNonceInvalid):
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
	default:
		shared.HandleError(w, err, http.StatusUnauthorized, err.Error())
	}
	return false
}

func isEmailVerified(w http.ResponseWriter, claims *oidchelper.GoogleClaims) bool {
	if !claims.EmailVerified {
		shared.SendErrorResponse(w, http.StatusUnauthorized, errors.New(string(constants.ErrOIDCEmailNotVerified)))
		return false
	}
	return true
}

func resolveOIDCUser(w http.ResponseWriter, claims *oidchelper.GoogleClaims) (*userresource.User, bool) {
	userClient := clients.GetUserClient()
	user, err := userClient.GetUserByIdentity(constants.IdentityProviderGoogle, claims.Issuer, claims.Subject)
	if err == nil {
		return refuseBootstrap(w, user)
	}
	if !isNotFoundError(err) {
		shared.HandleError(w, err, http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrOIDCIdentityLookupFailed),
				shared.IdentityHash(claims.Subject), err))
		return nil, false
	}
	user, err = jitProvisionUser(userClient, claims)
	if errors.Is(err, ErrEmailReserved) {
		refuseReserved(w, claims.Email)
		return nil, false
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrEmailAmbiguous) || errors.Is(err, ErrEmailAlreadyBound) {
			status = http.StatusConflict
		}
		sendCallbackError(w, fmt.Errorf(string(constants.ErrOIDCJITProvisioningFailed), err), status)
		return nil, false
	}
	return refuseBootstrap(w, user)
}

// Every Google login resolves here, by subject or by email: the bootstrap admin
// signs in with a passkey only, so an identity bound to it before it was promoted never logs in.
func refuseBootstrap(w http.ResponseWriter, user *userresource.User) (*userresource.User, bool) {
	if !user.Bootstrap {
		return user, true
	}
	refuseReserved(w, user.Email)
	return nil, false
}

func refuseReserved(w http.ResponseWriter, email string) {
	lg.Info(fmt.Sprintf(string(constants.LogOIDCBootstrapRefused), shared.IdentityHash(email)))
	shared.SendErrorResponse(w, http.StatusForbidden, ErrEmailReserved)
}

// A refusal answers the caller and is not logged; a failure on this side is.
func sendCallbackError(w http.ResponseWriter, err error, status int) {
	if status < http.StatusInternalServerError {
		shared.SendErrorResponse(w, status, err)
		return
	}
	shared.HandleError(w, err, status, err.Error())
}
