package auth

import (
	"errors"
	"fmt"
	"strings"

	authdata "github.com/telark/telark/internal/data/auth"
	dataerrors "github.com/telark/telark/internal/data/errors"
	userresource "github.com/telark/telark/internal/data/resources/user"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var lg = constants.GetLogger(constants.LoggerPrefixHelper)

type UserRetrievalFunc func(string) (*userresource.User, error)

func GetUserWithErrorHandling(
	identifier string,
	retrievalFunc UserRetrievalFunc,
) (*userresource.User, error) {
	user, err := retrievalFunc(identifier)
	if err != nil {
		if errors.Is(err, restshared.ErrNotFound) {
			return nil, shared.Refuse(constants.RefusalUserNotFound, errors.New(string(constants.ErrUserNotFound)))
		}
		if errors.Is(err, restshared.ErrGone) {
			return nil, shared.Refuse(constants.RefusalAccountNotActive, errors.New(string(dataerrors.ErrAuthzUserNotActive)))
		}
		lg.Error(fmt.Sprintf(string(constants.ErrFailedGetUser), shared.IdentityHash(identifier), err))
		return nil, shared.ErrBackendUnavailable
	}
	if user == nil {
		return nil, shared.Refuse(constants.RefusalUserNotFound, errors.New(string(constants.ErrUserNotFound)))
	}
	return user, nil
}

func GetUserByIDWithErrorHandling(userID string) (*userresource.User, error) {
	userClient := clients.GetUserClient()
	return GetUserWithErrorHandling(userID, userClient.GetUserByID)
}

func GetUserAndPasskeys(email string) (*userresource.User, []*authdata.Passkey, error) {
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
