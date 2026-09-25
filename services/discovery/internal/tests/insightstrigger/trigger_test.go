package insightstrigger

import (
	"context"
	"strconv"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/insights"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	appName      = "api"
	appNamespace = "prod"
	genOld       = 5
	genPlain     = 6
	genNew       = 7
)

func publisherEnv(t *testing.T) *goredis.Client {
	t.Helper()
	mr := testutil.RedisEnv(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		insights.Default = nil
		if err := rdb.Close(); err != nil {
			t.Logf("close redis client: %v", err)
		}
	})
	insights.Init(rdb, constants.GetLogger(constants.LoggerPrefixDiscoveryManager))
	return rdb
}

func app(entries ...application.ChangeLogEntry) *application.Application {
	return &application.Application{
		Name:       appName,
		Namespaces: application.Namespaces{Items: []application.NamespaceEntry{{Name: appNamespace}}},
		History:    application.ApplicationHistory{ChangeLog: entries},
	}
}

func jobs(t *testing.T, rdb *goredis.Client) []goredis.XMessage {
	t.Helper()
	msgs, err := rdb.XRange(context.Background(), insightsdata.StreamJobs, "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	return msgs
}

func assertOneJob(t *testing.T, rdb *goredis.Client, trigger string, generation int) {
	t.Helper()
	msgs := jobs(t, rdb)
	testutil.Equal(t, "xlen", len(msgs), constants.DefaultAddValue)
	values := msgs[constants.DefaultInitValue].Values
	testutil.Equal(t, "field count", len(values), len([]string{
		insightsdata.FieldNamespace, insightsdata.FieldName, insightsdata.FieldTrigger, insightsdata.FieldGeneration,
	}))
	testutil.Equal(t, insightsdata.FieldNamespace, values[insightsdata.FieldNamespace], any(appNamespace))
	testutil.Equal(t, insightsdata.FieldName, values[insightsdata.FieldName], any(appName))
	testutil.Equal(t, insightsdata.FieldTrigger, values[insightsdata.FieldTrigger], any(trigger))
	testutil.Equal(t, insightsdata.FieldGeneration, values[insightsdata.FieldGeneration], any(strconv.Itoa(generation)))
}

func TestEnqueueIncident(t *testing.T) {
	rdb := publisherEnv(t)
	insights.Enqueue(app(application.ChangeLogEntry{Generation: genNew, IsIncident: true}))
	assertOneJob(t, rdb, insightsdata.TriggerIncident, genNew)
}

// The newest entry by generation decides, whatever the slice order.
func TestEnqueueRecovery(t *testing.T) {
	rdb := publisherEnv(t)
	insights.Enqueue(app(
		application.ChangeLogEntry{Generation: genOld, IsIncident: true},
		application.ChangeLogEntry{Generation: genNew, IsRecovery: true},
		application.ChangeLogEntry{Generation: genPlain},
	))
	assertOneJob(t, rdb, insightsdata.TriggerRecovery, genNew)
}

// An older incident does not trigger when the newest entry is a plain change.
func TestEnqueueSkipsPlainChange(t *testing.T) {
	rdb := publisherEnv(t)
	insights.Enqueue(app(
		application.ChangeLogEntry{Generation: genOld, IsIncident: true},
		application.ChangeLogEntry{Generation: genNew},
	))
	insights.Enqueue(app())
	testutil.Equal(t, "xlen", len(jobs(t, rdb)), constants.DefaultInitValue)
}

// Neither an uninitialized publisher nor an unreachable Redis may panic the publish path.
func TestEnqueueNilPublisherNoop(t *testing.T) {
	insights.Default = nil
	insights.Enqueue(app(application.ChangeLogEntry{Generation: genNew, IsIncident: true}))

	mr := testutil.RedisEnv(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		if err := rdb.Close(); err != nil {
			t.Logf("close redis client: %v", err)
		}
	})
	mr.Close()
	insights.NewPublisher(rdb, constants.GetLogger(constants.LoggerPrefixDiscoveryManager)).
		Enqueue(context.Background(), app(application.ChangeLogEntry{Generation: genNew, IsIncident: true}))
}
