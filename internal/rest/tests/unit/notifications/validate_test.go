package notifications_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/plsyro/rest/clients/notifications"
)

func validNotification() notifications.Notification {
	return notifications.Notification{
		UserID:   "user-1",
		Type:     notifications.TypeRoleChanged,
		Title:    "Your roles were updated",
		Message:  "Granted Owner role",
		Severity: notifications.SeverityInfo,
	}
}

func TestValidateForEmit_AcceptsValid(t *testing.T) {
	n := validNotification()
	if err := notifications.ValidateForEmit(&n); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateForEmit_RejectsMissingUserID(t *testing.T) {
	n := validNotification()
	n.UserID = ""
	err := notifications.ValidateForEmit(&n)
	if !errors.Is(err, notifications.ErrUserIDRequired) {
		t.Fatalf("expected ErrUserIDRequired, got %v", err)
	}
}

func TestValidateForEmit_RejectsMissingType(t *testing.T) {
	n := validNotification()
	n.Type = ""
	err := notifications.ValidateForEmit(&n)
	if !errors.Is(err, notifications.ErrTypeRequired) {
		t.Fatalf("expected ErrTypeRequired, got %v", err)
	}
}

func TestValidateForEmit_RejectsMissingTitle(t *testing.T) {
	n := validNotification()
	n.Title = ""
	err := notifications.ValidateForEmit(&n)
	if !errors.Is(err, notifications.ErrTitleRequired) {
		t.Fatalf("expected ErrTitleRequired, got %v", err)
	}
}

func TestValidateForEmit_RejectsMissingMessage(t *testing.T) {
	n := validNotification()
	n.Message = ""
	err := notifications.ValidateForEmit(&n)
	if !errors.Is(err, notifications.ErrMessageRequired) {
		t.Fatalf("expected ErrMessageRequired, got %v", err)
	}
}

func TestValidateForEmit_RejectsInvalidSeverity(t *testing.T) {
	n := validNotification()
	n.Severity = "critical"
	err := notifications.ValidateForEmit(&n)
	if !errors.Is(err, notifications.ErrSeverityInvalid) {
		t.Fatalf("expected ErrSeverityInvalid, got %v", err)
	}
}

func TestValidateForEmit_AcceptsAllSeverities(t *testing.T) {
	severities := []string{
		notifications.SeverityInfo,
		notifications.SeveritySuccess,
		notifications.SeverityWarning,
		notifications.SeverityError,
	}
	for _, s := range severities {
		n := validNotification()
		n.Severity = s
		if err := notifications.ValidateForEmit(&n); err != nil {
			t.Fatalf("severity %q: expected nil error, got %v", s, err)
		}
	}
}

func TestTruncate_TitleAndMessage(t *testing.T) {
	n := notifications.Notification{
		Title:   strings.Repeat("a", notifications.MaxTitleLen+50),
		Message: strings.Repeat("b", notifications.MaxMessageLen+200),
	}
	notifications.Truncate(&n)
	if len(n.Title) != notifications.MaxTitleLen {
		t.Fatalf("expected title len %d, got %d", notifications.MaxTitleLen, len(n.Title))
	}
	if len(n.Message) != notifications.MaxMessageLen {
		t.Fatalf("expected message len %d, got %d", notifications.MaxMessageLen, len(n.Message))
	}
}

func TestTruncate_NoOpWhenWithinLimits(t *testing.T) {
	n := notifications.Notification{
		Title:   "short",
		Message: "also short",
	}
	notifications.Truncate(&n)
	if n.Title != "short" || n.Message != "also short" {
		t.Fatalf("truncate altered values within limits: %+v", n)
	}
}
