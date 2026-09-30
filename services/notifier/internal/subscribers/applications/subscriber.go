package applications

import (
	"github.com/nats-io/nats.go"
	appresource "github.com/telark/telark/internal/data/resources/application"
	resourceshared "github.com/telark/telark/internal/data/resources/shared"
	applicationsclient "github.com/telark/telark/internal/rest/clients/applications"
	"github.com/telark/telark/internal/rest/response"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/notifier/internal/subscribers/base"
)

// Narrow slice of the applications client so tests can inject a fake and cover
// patch / create-on-404 against the exporter and reset against discovery.
type AppClient interface {
	PatchApplicationByName(name string, body map[string]any) *response.GenericResponse
	CreateApplication(app *appresource.Application) *response.GenericResponse
	ResetApplicationByName(name string) (*response.GenericResponse, error)
}

type ApplicationSubscriber struct {
	*base.BaseSubscriber
	client AppClient
}

func NewApplicationSubscriber() natscore.ResourceSubscriber {
	return NewApplicationSubscriberWithClient(applicationsclient.NewClient())
}

// Returns the concrete type, not the interface, so tests can tune retry timings.
func NewApplicationSubscriberWithClient(client AppClient) *ApplicationSubscriber {
	subscriber := &ApplicationSubscriber{
		BaseSubscriber: base.NewBaseSubscriber(natscore.Applications, resourceshared.Application),
		client:         client,
	}

	subscriber.SetHandlerCallback(subscriber.executeHandler)
	return subscriber
}

func (s *ApplicationSubscriber) executeHandler(m *nats.Msg, action natscore.Action) error {
	return base.SharedExecuteHandler(m, action, s.getHandler)
}
