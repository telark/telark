package session

import (
	authdata "github.com/telark/data/auth"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/auth/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ValidateSessionExpiration(resource *unstructured.Unstructured) (*authdata.Session, error) {
	session, err := UnstructuredToSession(resource)
	if err != nil {
		return nil, err
	}

	if session.ExpiresTimestamp == constants.EmptyString {
		return session, nil
	}

	if err := shared.ValidateExpiresAt(session.ExpiresTimestamp, constants.ErrSessionExpired); err != nil {
		return nil, err
	}

	return session, nil
}
