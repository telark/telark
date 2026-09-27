package core

import (
	"slices"
	"time"

	"github.com/telark/data/resources/application"
	datashared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/constants"
)

func MarkPublished(app *application.Application) {
	setPublishedCondition(app, datashared.ConditionTrue, application.ConditionReasonCreated)
}

func MarkPublishFailed(app *application.Application) {
	setPublishedCondition(app, datashared.ConditionFalse, application.ConditionReasonFailed)
}

func PublishedCondition(app *application.Application) *application.Condition {
	idx := slices.IndexFunc(app.Conditions, func(c application.Condition) bool {
		return c.Type == application.ConditionTypePublished
	})
	if idx < constants.DefaultInitValue {
		return nil
	}
	return &app.Conditions[idx]
}

func IsPublished(app *application.Application) bool {
	c := PublishedCondition(app)
	return c != nil && c.Status == datashared.ConditionTrue
}

func IsPublishFailed(app *application.Application) bool {
	c := PublishedCondition(app)
	return c != nil && c.Reason == application.ConditionReasonFailed
}

func setPublishedCondition(app *application.Application, status datashared.ConditionStatus, reason string) {
	current := PublishedCondition(app)
	if current != nil && current.Status == status && current.Reason == reason {
		return
	}
	next := application.Condition{
		Type:               application.ConditionTypePublished,
		Status:             status,
		Reason:             reason,
		LastTransitionTime: time.Now().UTC().Format(time.RFC3339),
	}
	if current != nil {
		*current = next
		return
	}
	app.Conditions = append(app.Conditions, next)
}
