package session

import (
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/services/exporter/internal/constants"
	authutils "github.com/telark/telark/services/exporter/internal/utils/auth/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractSessionSpec(body map[string]any, userID string) (*authdata.Session, string, error) {
	session, err := sharedutils.ExtractStructFromBody[authdata.Session](body)
	if err != nil {
		return nil, constants.EmptyString, err
	}

	if err := validateSessionFields(session, userID); err != nil {
		return nil, constants.EmptyString, err
	}

	sessionName := SessionName(session.SessionToken)
	session.SessionToken = constants.EmptyString

	return session, sessionName, nil
}

func validateSessionFields(session *authdata.Session, userID string) error {
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

func UnstructuredToSession(resource *unstructured.Unstructured) (*authdata.Session, error) {
	return sharedutils.UnstructuredToStruct[authdata.Session](
		resource,
		constants.ErrSessionSpecNotFound,
		constants.ErrSessionSpecInvalid,
		constants.ErrFailedToUnmarshalSession,
	)
}
