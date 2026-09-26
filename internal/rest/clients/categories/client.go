package categories

import (
	"net/url"

	"github.com/telark/data/classification/category"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/categories"
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
