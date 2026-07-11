package notifications

import (
	"errors"

	"github.com/telark/exporter/internal/constants"
)

var (
	ErrUserIDRequired   = errors.New("notification: userID is required")
	ErrTypeRequired     = errors.New("notification: type is required")
	ErrTitleRequired    = errors.New("notification: title is required")
	ErrMessageRequired  = errors.New("notification: message is required")
	ErrSeverityInvalid  = errors.New("notification: severity must be info, success, warning, or error")
)

func ValidateForEmit(n *Notification) error {
	if n.UserID == constants.EmptyString {
		return ErrUserIDRequired
	}
	if n.Type == constants.EmptyString {
		return ErrTypeRequired
	}
	if n.Title == constants.EmptyString {
		return ErrTitleRequired
	}
	if n.Message == constants.EmptyString {
		return ErrMessageRequired
	}
	switch n.Severity {
	case SeverityInfo, SeveritySuccess, SeverityWarning, SeverityError:
	default:
		return ErrSeverityInvalid
	}
	return nil
}

func Truncate(n *Notification) {
	if len(n.Title) > MaxTitleLen {
		n.Title = n.Title[:MaxTitleLen]
	}
	if len(n.Message) > MaxMessageLen {
		n.Message = n.Message[:MaxMessageLen]
	}
}
