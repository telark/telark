package clients

import (
	"context"
	"time"

	"github.com/telark/discovery/internal/constants"
	notifclient "github.com/telark/rest/clients/notifications"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/response"
)

const notifEmitTimeout = 3 * time.Second

type NotificationClient struct {
	client *notifclient.Client
}

func NewNotificationClient() *NotificationClient {
	cfg := &shared.ClientConfig{Timeout: notifEmitTimeout}
	return &NotificationClient{
		client: notifclient.NewClientWithConfig(cfg),
	}
}

func (c *NotificationClient) Emit(ctx context.Context, n notifclient.Notification) error {
	return guardedStatusError(constants.ErrNotificationEmitFailed, func() *response.GenericResponse {
		return c.client.Emit(ctx, n)
	})
}
