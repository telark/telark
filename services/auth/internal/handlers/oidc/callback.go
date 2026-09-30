package oidc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	telarkconfigresource "github.com/telark/data/resources/telarkconfig"
	userresource "github.com/telark/data/resources/user"
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
		shared.HandleError(w, err, http.StatusUnauthorized, err.Error())
		return
	}

	if err := oidchelper.VerifyAndConsumeNonce(claims.Nonce); err != nil {
		shared.HandleError(w, err, http.StatusUnauthorized, err.Error())
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
		shared.HandleError(w, fmt.Errorf(string(constants.ErrFailedCreateSession), err),
			shared.GetStatusCodeForSessionError(err),
			fmt.Sprintf(string(constants.ErrFailedCreateSession), err))
		return
	}

	capturedUserID, capturedPhase := user.ID, user.Status.Phase
	authhelper.Dispatch(func() {
		authhelper.UpdateUserLastLogin(capturedUserID, capturedPhase)
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

func isEmailVerified(w http.ResponseWriter, claims *oidchelper.GoogleClaims) bool {
	if !claims.EmailVerified {
		shared.HandleError(w, errors.New(string(constants.ErrOIDCEmailNotVerified)),
			http.StatusUnauthorized, string(constants.ErrOIDCEmailNotVerified))
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
		shared.HandleError(w,
			fmt.Errorf(string(constants.ErrOIDCJITProvisioningFailed), err),
			status,
			fmt.Sprintf(string(constants.ErrOIDCJITProvisioningFailed), err))
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
	shared.HandleError(w, ErrEmailReserved, http.StatusForbidden,
		fmt.Sprintf(string(constants.LogOIDCBootstrapRefused), shared.IdentityHash(email)))
}
