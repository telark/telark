package oidc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
)

var lg = constants.GetLogger(constants.LoggerPrefixOIDC)

func GoogleCallback(w http.ResponseWriter, r *http.Request) {
	cfg, err := shared.GetCachedConfig()
	if err != nil {
		shared.HandleError(w, fmt.Errorf(string(constants.ErrFailedGetConfig), err),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrFailedGetConfig), err))
		return
	}

	if !isOIDCConfigured(w, cfg) {
		return
	}

	var req CallbackRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	claims, err := oidchelper.ValidateGoogleIDToken(req.IDToken, cfg.OIDC.GoogleClientID)
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

	if config.IsBootstrapAdmin(claims.Email) {
		promoteBootstrapAdmin(user, clients.GetUserClient())
	}

	sessionToken, err := authhelper.CreateUserSession(user.ID, &req.DeviceMetadata)
	if err != nil {
		shared.HandleError(w, fmt.Errorf(string(constants.ErrFailedCreateSession), err),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrFailedCreateSession), err))
		return
	}

	lg.Info("OIDC login for " + claims.Email)
	shared.SendSuccessResponse(w, string(constants.SuccessOIDCLoginCompleted), CallbackResponse{
		SessionToken: sessionToken,
		Email:        claims.Email,
		User:         user,
	})
}

func isOIDCConfigured(w http.ResponseWriter, cfg *config.Config) bool {
	if cfg.OIDC.GoogleClientID == constants.EmptyString {
		shared.HandleError(w, errors.New(string(constants.ErrOIDCNotConfigured)),
			http.StatusServiceUnavailable, string(constants.ErrOIDCNotConfigured))
		return false
	}
	if !cfg.OIDC.EgressAllowed && cfg.OIDC.JWKJson == constants.EmptyString {
		shared.HandleError(w, errors.New(string(constants.ErrOIDCNotConfigured)),
			http.StatusServiceUnavailable, string(constants.ErrOIDCNotConfigured))
		return false
	}
	return true
}

func isEmailVerified(w http.ResponseWriter, claims *oidchelper.GoogleClaims) bool {
	if !claims.EmailVerified {
		shared.HandleError(w, errors.New(string(constants.ErrOIDCEmailNotVerified)),
			http.StatusUnauthorized, string(constants.ErrOIDCEmailNotVerified))
		return false
	}
	return true
}

func resolveOIDCUser(w http.ResponseWriter, claims *oidchelper.GoogleClaims) (*userresource.UserAsResource, bool) {
	userClient := clients.GetUserClient()
	user, err := userClient.GetUserByIdentity("google", claims.Issuer, claims.Subject)
	if err == nil {
		return user, true
	}
	if !isNotFoundError(err) {
		shared.HandleError(w, err, http.StatusInternalServerError,
			fmt.Sprintf("user identity lookup failed for %s: %v", claims.Email, err))
		return nil, false
	}
	user, err = jitProvisionUser(userClient, claims)
	if err != nil {
		shared.HandleError(w,
			fmt.Errorf(string(constants.ErrOIDCJITProvisioningFailed), err),
			http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrOIDCJITProvisioningFailed), err))
		return nil, false
	}
	return user, true
}
