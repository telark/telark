package core

import (
	"encoding/json"
	"time"

	"github.com/telark/data/resources/application"
	resourceshared "github.com/telark/data/resources/shared"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/publisher"
	natscore "github.com/telark/x-ware/nats/core"
)

func PublishApplications(natsClient *natscore.NATSClient, apps []application.Application) {
	if natsClient == nil {
		return
	}
	attemptMax := config.SnapshotWriteMaxAttempts()
	interval := config.SnapshotWriteRetryInterval()
	for i := range apps {
		app := &apps[i]
		snapshot.NormalizeApplicationSnapshotTakenAt(app)
		payload := applicationPayload(app)
		params := publisher.PublishUpdateParams{
			Name:    app.Name,
			Scope:   resourceshared.ApplicationSpecScope,
			Group:   natscore.Applications,
			Data:    payload,
			ResType: resourceshared.Application,
		}
		var lastErr error
		for attempt := constants.DefaultAddValue; attempt <= attemptMax; attempt++ {
			lastErr = publisher.PublishUpdate(params, natsClient)
			if lastErr == nil {
				app.CRStatus = application.CRStatusPublished
				break
			}
			if attempt < attemptMax {
				time.Sleep(interval)
			}
		}
		if lastErr != nil {
			app.CRStatus = application.CRStatusFailed
		}
	}
}

func applicationPayload(app *application.Application) map[string]any {
	b, err := json.Marshal(app)
	if err != nil {
		return map[string]any{payloadKeyName: app.Name}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{payloadKeyName: app.Name}
	}
	replaceSnapshotsWithExplicitTakenAt(m, app)
	return m
}
