package notifications

import "errors"

var (
	ErrUserIDRequired         = errors.New("notification: userID is required")
	ErrTypeRequired           = errors.New("notification: type is required")
	ErrTitleRequired          = errors.New("notification: title is required")
	ErrMessageRequired        = errors.New("notification: message is required")
	ErrSeverityInvalid        = errors.New("notification: severity must be info, success, warning, or error")
	ErrNotificationIDRequired = errors.New("notification: notification ID is required")
)
