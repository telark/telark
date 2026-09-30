package coreapps

import (
	"testing"

	"github.com/telark/data/resources/application"
	datashared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/tests/testutil"
)

// One Published entry per application, flipped in place rather than appended.
func TestPublishedCondition(t *testing.T) {
	app := &application.Application{}
	testutil.Equal(t, "no condition yet", core.PublishedCondition(app) == nil, true)
	testutil.Equal(t, "not published", core.IsPublished(app), false)
	testutil.Equal(t, "not failed", core.IsPublishFailed(app), false)

	core.MarkPublishFailed(app)
	testutil.Equal(t, "failed", core.IsPublishFailed(app), true)
	testutil.Equal(t, "failed status", core.PublishedCondition(app).Status, datashared.ConditionFalse)
	testutil.Equal(t, "failed reason", core.PublishedCondition(app).Reason, application.ConditionReasonFailed)

	core.MarkPublished(app)
	testutil.Equal(t, "one condition", len(app.Conditions), constants.DefaultAddValue)
	testutil.Equal(t, "published", core.IsPublished(app), true)
	testutil.Equal(t, "no longer failed", core.IsPublishFailed(app), false)
	testutil.Equal(t, "type", app.Conditions[constants.DefaultInitValue].Type, application.ConditionTypePublished)
	testutil.Equal(t, "created reason", app.Conditions[constants.DefaultInitValue].Reason, application.ConditionReasonCreated)
}

// The payload was built before MarkPublished, so no CR ever carried the condition and the
// UI Published row never rendered; app itself must still read unpublished until the outcome.
func TestPublishedPayloadCarriesCondition(t *testing.T) {
	app := &application.Application{Name: "shop"}
	payload := core.PublishedPayload(app)

	conditions, ok := payload["conditions"].([]any)
	testutil.Equal(t, "payload has conditions", ok, true)
	testutil.Equal(t, "one condition", len(conditions), constants.DefaultAddValue)
	published, ok := conditions[constants.DefaultInitValue].(map[string]any)
	testutil.Equal(t, "condition is an object", ok, true)
	testutil.Equal(t, "type", published["type"], any(application.ConditionTypePublished))
	testutil.Equal(t, "status", published["status"], any(string(datashared.ConditionTrue)))
	testutil.Equal(t, "in-memory app untouched", core.PublishedCondition(app) == nil, true)
}
