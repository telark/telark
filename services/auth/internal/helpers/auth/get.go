package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
	authdata "github.com/telark/data/auth"
	userresource "github.com/telark/data/resources/user"
)

var lg = constants.GetLogger(constants.LoggerPrefixHelper)

type UserRetrievalFunc func(string) (*userresource.UserAsResource, error)

func GetUserWithErrorHandling(
	identifier string,
	retrievalFunc UserRetrievalFunc,
) (*userresource.UserAsResource, error) {
	user, err := retrievalFunc(identifier)
	if err != nil {
		if strings.Contains(err.Error(), constants.HTTPStatus404Pattern) {
			return nil, errors.New(string(constants.ErrUserNotFound))
		}
		return nil, fmt.Errorf(string(constants.ErrFailedGetUser),
			shared.IdentityHash(identifier), err.Error())
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
		return nil, nil, errors.New(string(constants.ErrInternalServerError))
	}

	if passkeys == nil || len(passkeys) == constants.InitialCapacity {
		return nil, nil, errors.New(string(constants.ErrNoPasskeysFound))
	}

	return user, passkeys, nil
}
