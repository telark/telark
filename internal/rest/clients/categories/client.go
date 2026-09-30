package categories

import (
	"net/url"

	"github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/categories"
)

type Client struct {
	*shared.Client
}

func NewClient() *Client {
	return &Client{Client: shared.New(base.Exporter)}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{Client: shared.NewWithConfig(base.Exporter, cfg)}
}

func (c *Client) GetCategoriesByScope(scope string) ([]category.Category, error) {
	query := url.Values{eps.QueryScope: []string{scope}}
	ep := base.Endpoint(string(eps.GetAllCategories) + constants.QuerySeparator + query.Encode())
	return shared.GetListTyped[category.Category](c.Client, ep)
}
