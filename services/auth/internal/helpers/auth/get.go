package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
	dataerrors "github.com/telark/data/errors"
	userresource "github.com/telark/data/resources/user"
	restshared "github.com/telark/rest/clients/shared"
)

var lg = constants.GetLogger(constants.LoggerPrefixHelper)

type UserRetrievalFunc func(string) (*userresource.UserAsResource, error)

func GetUserWithErrorHandling(
	identifier string,
	retrievalFunc UserRetrievalFunc,
) (*userresource.UserAsResource, error) {
	user, err := retrievalFunc(identifier)
	if err != nil {
		if errors.Is(err, restshared.ErrNotFound) {
			return nil, errors.New(string(constants.ErrUserNotFound))
		}
		if errors.Is(err, restshared.ErrGone) {
			return nil, errors.New(string(dataerrors.ErrAuthzUserNotActive))
		}
		lg.Error(fmt.Sprintf(string(constants.ErrFailedGetUser), shared.IdentityHash(identifier), err))
		return nil, shared.ErrBackendUnavailable
	}
	if user == nil {
		return nil, errors.New(string(constants.ErrUserNotFound))
	}
	return user, nil
}

func GetUserByIDWithErrorHandling(userID string) (*userresource.UserAsResource, error) {
	userClient := clients.GetUserClient()
	return GetUserWithErrorHandling(userID, userClient.GetUserByID)
}

func GetUserAndPasskeys(email string) (*userresource.UserAsResource, []*authdata.UserPasskey, error) {
	userClient := clients.GetUserClient()
	user, err := GetUserWithErrorHandling(email, userClient.GetUserByEmail)
	if err != nil {
		return nil, nil, err
	}

	passkeyClient := clients.GetPasskeyClient()
	passkeys, err := passkeyClient.GetAllPasskeysByUser(user.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), string(constants.ErrPasskeyNotFound)) {
			return nil, nil, errors.New(string(constants.ErrNoPasskeysFound))
		}
		lg.Error(fmt.Sprintf(string(constants.ErrFailedGetPasskeys), err))
		return nil, nil, shared.ErrBackendUnavailable
	}

	if passkeys == nil || len(passkeys) == constants.InitialCapacity {
		return nil, nil, errors.New(string(constants.ErrNoPasskeysFound))
	}

	return user, passkeys, nil
}
