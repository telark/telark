package notifications

import (
	"context"
	"fmt"
	"net/url"

	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/notifications"
	"github.com/telark/rest/response"
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
	if userID == constants.EmptyString {
		return nil, ErrUserIDRequired
	}
	limit := opts.Limit
	if limit <= constants.EmptySliceLength {
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
	if userID == constants.EmptyString {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	if notificationID == constants.EmptyString {
		return shared.CreateErrorResponse(ErrNotificationIDRequired.Error(), ErrNotificationIDRequired)
	}
	ep := base.Endpoint(fmt.Sprintf(userIDQueryFormat, eps.MarkRead, url.QueryEscape(userID)))
	byID := c.WithParams(map[string]string{constants.IDParam: notificationID})
	return byID.Update(ep, map[string]any{})
}

func (c *Client) MarkAllRead(ctx context.Context, userID string) *response.GenericResponse {
	_ = ctx
	if userID == constants.EmptyString {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	ep := base.Endpoint(fmt.Sprintf(userIDQueryFormat, eps.MarkAllRead, url.QueryEscape(userID)))
	resp, err := c.Post(ep)
	if err != nil {
		return shared.CreateErrorResponse(err.Error(), err)
	}
	return resp
}

func (c *Client) Clear(ctx context.Context, userID string) *response.GenericResponse {
	_ = ctx
	if userID == constants.EmptyString {
		return shared.CreateErrorResponse(ErrUserIDRequired.Error(), ErrUserIDRequired)
	}
	ep := base.Endpoint(fmt.Sprintf(userIDQueryFormat, eps.Clear, url.QueryEscape(userID)))
	return c.Delete(ep)
}

func ValidateForEmit(n *Notification) error {
	if n.UserID == constants.EmptyString {
		return ErrUserIDRequired
	}
	if n.Type == constants.EmptyString {
		return ErrTypeRequired
	}
	if n.Title == constants.EmptyString {
		return ErrTitleRequired
	}
	if n.Message == constants.EmptyString {
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
