package notifications

import (
	"context"
	"fmt"
	"net/url"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/notifications"
	"github.com/plsyro/rest/response"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Exporter, cfg),
	}
}

func (c *Client) Emit(ctx context.Context, n Notification) *response.GenericResponse {
	_ = ctx
	if err := ValidateForEmit(&n); err != nil {
		return shared.CreateErrorResponse(err.Error(), err)
	}
	Truncate(&n)
	return shared.ExecuteRequestWithHeaders(c.Client, base.Post, eps.Emit, n, nil)
}

func (c *Client) List(ctx context.Context, userID string, opts ListOptions) (*ListResponse, error) {
	_ = ctx
	if userID == "" {
		return nil, ErrUserIDRequired
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	ep := base.Endpoint(fmt.Sprintf("%s?userId=%s&limit=%d&cursor=%s",
		eps.List,
		url.QueryEscape(userID),
		limit,
		url.QueryEscape(opts.Cursor),
	))
	return shared.GetTyped[ListResponse](c.Client, ep)
}

func (c *Client) MarkRead(ctx context.Context, userID, notificationID string) *response.GenericResponse {
	_ = ctx
	if userID == "" {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	if notificationID == "" {
		return shared.CreateErrorResponse(ErrNotificationIDRequired.Error(), ErrNotificationIDRequired)
	}
	ep := shared.SubstituteEndpointWithParam(string(eps.MarkRead), constants.IDParam, notificationID)
	ep = base.Endpoint(fmt.Sprintf("%s?userId=%s", ep, url.QueryEscape(userID)))
	return c.Update(ep, map[string]any{})
}

func (c *Client) MarkAllRead(ctx context.Context, userID string) *response.GenericResponse {
	_ = ctx
	if userID == "" {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	ep := base.Endpoint(fmt.Sprintf("%s?userId=%s", eps.MarkAllRead, url.QueryEscape(userID)))
	resp, err := c.Post(ep)
	if err != nil {
		return shared.CreateErrorResponse(err.Error(), err)
	}
	return resp
}

func (c *Client) Clear(ctx context.Context, userID string) *response.GenericResponse {
	_ = ctx
	if userID == "" {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	ep := base.Endpoint(fmt.Sprintf("%s?userId=%s", eps.Clear, url.QueryEscape(userID)))
	return c.Delete(ep)
}

func ValidateForEmit(n *Notification) error {
	if n.UserID == "" {
		return ErrUserIDRequired
	}
	if n.Type == "" {
		return ErrTypeRequired
	}
	if n.Title == "" {
		return ErrTitleRequired
	}
	if n.Message == "" {
		return ErrMessageRequired
	}
	switch n.Severity {
	case SeverityInfo, SeveritySuccess, SeverityWarning, SeverityError:
	default:
		return ErrSeverityInvalid
	}
	return nil
}

func Truncate(n *Notification) {
	if len(n.Title) > MaxTitleLen {
		n.Title = n.Title[:MaxTitleLen]
	}
	if len(n.Message) > MaxMessageLen {
		n.Message = n.Message[:MaxMessageLen]
	}
}
