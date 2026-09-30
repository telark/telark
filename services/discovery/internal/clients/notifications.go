package clients

import (
	"context"
	"time"

	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/constants"
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
