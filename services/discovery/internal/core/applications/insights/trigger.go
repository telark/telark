package insights

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	goredis "github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

// Default stays nil until Init runs, which turns Enqueue into a no-op.
var Default *Publisher

func Init(rdb *goredis.Client, lg Logger) {
	Default = NewPublisher(rdb, lg)
}

func NewPublisher(rdb *goredis.Client, lg Logger) *Publisher {
	return &Publisher{stream: xwareredis.NewStreamClient(rdb), lg: lg}
}

func Enqueue(app *application.Application) {
	if Default == nil {
		return
	}
	Default.Enqueue(context.Background(), app)
}

// Enqueue is best-effort: the analyzer is optional, so a failed XADD is only logged.
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
	ctx, cancel := context.WithTimeout(ctx, constants.InsightsTriggerTimeout)
	defer cancel()
	_, err := p.stream.PublishWithMaxLen(ctx, insightsdata.StreamJobs, map[string]any{
		insightsdata.FieldNamespace:  ns,
		insightsdata.FieldName:       app.Name,
		insightsdata.FieldTrigger:    trigger,
		insightsdata.FieldGeneration: entry.Generation,
	}, insightsdata.StreamMaxLen)
	if err != nil {
		p.lg.Warn(fmt.Sprintf(string(constants.WarnInsightsTriggerFailed), app.Name, err))
	}
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
