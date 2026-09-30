package insights

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	goredis "github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/telark/internal/data/insights"
	"github.com/telark/telark/internal/data/resources/application"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
)

// Default stays nil until Init runs, which turns Enqueue into a no-op.
var Default *Publisher

func Init(rdb *goredis.Client, lg Logger) {
	Default = NewPublisher(rdb, lg, clients.NewExporterClient().GetApplicationByNameFresh)
}

func NewPublisher(rdb *goredis.Client, lg Logger, stored StoredApplicationFn) *Publisher {
	return &Publisher{stream: xwareredis.NewStreamClient(rdb), lg: lg, stored: stored}
}

func Enqueue(app *application.Application) {
	if Default == nil {
		return
	}
	Default.Enqueue(context.Background(), app)
}

// Enqueue is best-effort: the analyzer is optional, so a failed XADD is only logged.
// The publish lands through NATS and the notifier, so the XADD waits off this path
// until the exporter serves the entry's generation: the analyzer reads the app from there.
func (p *Publisher) Enqueue(ctx context.Context, app *application.Application) {
	entries := app.History.ChangeLog
	if len(entries) == constants.DefaultInitValue || len(app.Namespaces.Items) == constants.DefaultInitValue {
		return
	}
	entry := slices.MaxFunc(entries, func(a, b application.ChangeLogEntry) int {
		return cmp.Compare(a.Generation, b.Generation)
	})
	trigger := triggerFor(&entry)
	ns := app.Namespaces.Items[constants.DefaultInitValue].Name
	if trigger == constants.EmptyString || ns == constants.EmptyString {
		return
	}
	fields := map[string]any{
		insightsdata.FieldNamespace:  ns,
		insightsdata.FieldName:       app.Name,
		insightsdata.FieldTrigger:    trigger,
		insightsdata.FieldGeneration: entry.Generation,
	}
	go p.publishOnceStored(ctx, app.Name, entry.Generation, fields)
}

func (p *Publisher) publishOnceStored(ctx context.Context, name string, generation int, fields map[string]any) {
	if !p.storeHasGeneration(ctx, name, generation) {
		waited := constants.InsightsEnqueueStoreWaitAttempts * constants.InsightsEnqueueStorePollInterval
		p.lg.Warn(fmt.Sprintf(string(constants.WarnInsightsStoreBehind), name, generation, waited))
	}
	ctx, cancel := context.WithTimeout(ctx, constants.InsightsTriggerTimeout)
	defer cancel()
	if _, err := p.stream.PublishWithMaxLen(ctx, insightsdata.StreamJobs, fields, insightsdata.StreamMaxLen); err != nil {
		p.lg.Warn(fmt.Sprintf(string(constants.WarnInsightsTriggerFailed), name, err))
	}
}

func (p *Publisher) storeHasGeneration(ctx context.Context, name string, generation int) bool {
	if p.stored == nil {
		return true
	}
	for attempt := constants.DefaultInitValue; attempt < constants.InsightsEnqueueStoreWaitAttempts; attempt++ {
		if app, err := p.stored(name); err == nil && app != nil && app.History.Generation >= generation {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(constants.InsightsEnqueueStorePollInterval):
		}
	}
	return false
}

func triggerFor(entry *application.ChangeLogEntry) string {
	switch {
	case entry.IsRecovery:
		return insightsdata.TriggerRecovery
	case entry.IsIncident:
		return insightsdata.TriggerIncident
	default:
		return constants.EmptyString
	}
}
