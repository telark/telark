package coreapps

import (
	"encoding/json"
	"testing"
	"time"

	natssrvtest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/telark/telark/internal/data/resources/application"
	datashared "github.com/telark/telark/internal/data/shared"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	natstreams "github.com/telark/telark/internal/x-ware/nats/streams"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
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

// Asks the embedded NATS server for an ephemeral port.
const randomPort = -1

const publishTimeout = 5 * time.Second

// An embedded JetStream server carrying the streams discovery publishes to in a cluster.
func natsClient(t *testing.T) *natscore.NATSClient {
	t.Helper()
	opts := natssrvtest.DefaultTestOptions
	opts.Port = randomPort
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	srv := natssrvtest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	conn, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	js, err := conn.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	client := &natscore.NATSClient{Conn: conn, JetStream: js}
	if err := natstreams.CreateStreams(client); err != nil {
		t.Fatal(err)
	}
	return client
}

// The payload was built before MarkPublished, so no CR ever carried the condition and the
// UI Published row never rendered; the app itself reads published only once the publish landed.
func TestPublishedPayloadCarriesCondition(t *testing.T) {
	client := natsClient(t)
	sub, err := client.Conn.SubscribeSync(natscore.GetTopicName(natscore.Applications, natscore.Update))
	if err != nil {
		t.Fatal(err)
	}
	apps := []application.Application{{Name: "shop"}}
	core.PublishApplications(client, apps, nil)

	msg, err := sub.NextMsg(publishTimeout)
	if err != nil {
		t.Fatal(err)
	}
	var published natscore.Message
	if err := json.Unmarshal(msg.Data, &published); err != nil {
		t.Fatal(err)
	}
	payload, ok := published.Data.(map[string]any)
	testutil.Equal(t, "payload is an object", ok, true)
	conditions, ok := payload["conditions"].([]any)
	testutil.Equal(t, "payload has conditions", ok, true)
	testutil.Equal(t, "one condition", len(conditions), constants.DefaultAddValue)
	condition, ok := conditions[constants.DefaultInitValue].(map[string]any)
	testutil.Equal(t, "condition is an object", ok, true)
	testutil.Equal(t, "type", condition["type"], any(application.ConditionTypePublished))
	testutil.Equal(t, "status", condition["status"], any(string(datashared.ConditionTrue)))
	testutil.Equal(t, "app published once the publish landed", core.IsPublished(&apps[constants.DefaultInitValue]), true)
}
