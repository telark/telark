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

func PublishApplications(natsClient *natscore.NATSClient, apps []application.Application, authored []bool) {
	if natsClient == nil {
		return
	}
	attemptMax := config.SnapshotWriteMaxAttempts()
	interval := config.SnapshotWriteRetryInterval()
	for i := range apps {
		app := &apps[i]
		snapshot.NormalizeApplicationSnapshotTakenAt(app)
		payload := applicationPayload(app)
		if i >= len(authored) || !authored[i] {
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

// stripUnauthoredHistory drops history and snapshots from a payload the caller
// did not author. The CR is written with a JSON merge patch, so omitting the
// keys leaves the stored values untouched; echoing back a possibly stale read
// would instead overwrite history authored by a concurrent flush.
func stripUnauthoredHistory(m map[string]any) {
	delete(m, payloadKeyHistory)
	delete(m, payloadKeySnapshots)
}
