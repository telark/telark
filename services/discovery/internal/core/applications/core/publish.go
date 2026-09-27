package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/telark/data/resources/application"
	resourceshared "github.com/telark/data/resources/shared"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/core/applications/insights"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/publisher"
	natscore "github.com/telark/x-ware/nats/core"
)

func PublishApplications(natsClient *natscore.NATSClient, apps []application.Application, outcomes []diff.Outcome) {
	if natsClient == nil {
		return
	}
	attemptMax := config.SnapshotWriteMaxAttempts()
	interval := config.SnapshotWriteRetryInterval()
	for i := range apps {
		app := &apps[i]
		if i < len(outcomes) && outcomes[i] == diff.OutcomeDeferred {
			continue
		}
		snapshot.NormalizeApplicationSnapshotTakenAt(app)
		payload := applicationPayload(app)
		authored := i < len(outcomes) && outcomes[i] == diff.OutcomeAuthored
		if !authored {
			stripUnauthoredHistory(payload)
		}
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
				MarkPublished(app)
				if authored {
					insights.Enqueue(app)
				}
				break
			}
			if attempt < attemptMax {
				time.Sleep(interval)
			}
		}
		if lastErr != nil {
			MarkPublishFailed(app)
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(string(constants.WarnApplicationPublishFailed), app.Name, attemptMax, lastErr))
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

// The CR is written with a JSON merge patch, so omitting the keys leaves the stored values
// untouched; echoing back a possibly stale read would overwrite a concurrent flush's history.
func stripUnauthoredHistory(m map[string]any) {
	delete(m, payloadKeyHistory)
	delete(m, payloadKeySnapshots)
}
