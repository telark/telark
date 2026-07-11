package session

import (
	"fmt"

	authdata "github.com/telark/data/auth"
	"github.com/telark/exporter/constants"
	authutils "github.com/telark/exporter/utils/auth/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractSessionSpec(body map[string]any, userID string) (*authdata.UserSession, string, error) {
	session, err := sharedutils.ExtractStructFromBody[authdata.UserSession](body)
	if err != nil {
		return nil, constants.EmptyString, err
	}

	if err := validateSessionFields(session, userID); err != nil {
		return nil, constants.EmptyString, err
	}

	sessionName, err := authutils.GenerateCRDName(userID, constants.ResourceTypeSession, FindSessionsByUserID)
	if err != nil {
		return nil, constants.EmptyString, fmt.Errorf(string(constants.ErrFailedToGenerateResourceName), err)
	}

	return session, sessionName, nil
}

func validateSessionFields(session *authdata.UserSession, userID string) error {
	if err := sharedutils.ValidateRequiredField(session.SessionToken, string(constants.ErrSessionFieldRequired)); err != nil {
		return err
	}

	session.UserID = userID

	if session.ExpiresTimestamp != constants.EmptyString {
		if err := authutils.ValidateExpiresAt(session.ExpiresTimestamp, constants.ErrSessionExpiresInPast); err != nil {
			return err
		}
	}

	return nil
}

func UnstructuredToSession(resource *unstructured.Unstructured) (*authdata.UserSession, error) {
	return sharedutils.UnstructuredToStruct[authdata.UserSession](
		resource,
		constants.ErrSessionSpecNotFound,
		constants.ErrSessionSpecInvalid,
		constants.ErrFailedToUnmarshalSession,
	)
}
