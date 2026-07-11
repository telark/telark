package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/discovery/internal/constants"
	notifclient "github.com/telark/rest/clients/notifications"
	"github.com/telark/rest/clients/shared"
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
	resp := c.client.Emit(ctx, n)
	if resp == nil {
		return errors.New(string(constants.ErrPatchApplicationReturnedNilResponse))
	}
	if resp.Status != http.StatusOK {
		return fmt.Errorf("emit notification: status=%d message=%s", resp.Status, resp.Message)
	}
	return nil
}
