package applications

import (
	"strings"

	"github.com/nats-io/nats.go"
	appresource "github.com/telark/data/resources/application"
	resourceshared "github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/subscribers/base"
	applicationsclient "github.com/telark/rest/clients/resources/applications"
	"github.com/telark/rest/response"
	natscore "github.com/telark/x-ware/nats/core"
)

// AppClient is the slice of the exporter applications client the subscriber
// needs. Defining it here lets tests inject a fake and exercise every branch
// (patch / create-on-404 / delete) without a live exporter.
type AppClient interface {
	PatchApplicationByName(name string, body map[string]any) *response.GenericResponse
	CreateApplication(app *appresource.Application) *response.GenericResponse
	DeleteApplicationByName(name string) *response.GenericResponse
}

type ApplicationSubscriber struct {
	*base.BaseSubscriber
	client AppClient
}

func NewApplicationSubscriber() natscore.ResourceSubscriber {
	return NewApplicationSubscriberWithClient(applicationsclient.NewClient())
}

// NewApplicationSubscriberWithClient builds a subscriber backed by the given
// exporter client — the seam used by tests (concrete type so retry timings can
// be tuned).
func NewApplicationSubscriberWithClient(client AppClient) *ApplicationSubscriber {
	subscriber := &ApplicationSubscriber{
		BaseSubscriber: base.NewBaseSubscriber(natscore.Applications, resourceshared.Application, nil),
		client:         client,
	}

	subscriber.SetHandlerCallback(subscriber.executeHandler)
	return subscriber
}

func (s *ApplicationSubscriber) executeHandler(m *nats.Msg, action natscore.Action) error {
	return base.SharedExecuteHandler(
		m,
		action,
		strings.ToLower(string(resourceshared.Application)),
		s.getHandler,
		nil,
	)
}
