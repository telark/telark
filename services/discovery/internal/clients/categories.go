package clients

import (
	"github.com/telark/data/classification/category"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/clients/categories"
	"github.com/telark/rest/clients/shared"
)

type CategoryClient struct {
	client *categories.Client
}

func NewCategoryClient() *CategoryClient {
	cfg := &shared.ClientConfig{Timeout: constants.CategoryReadTimeout}
	return &CategoryClient{client: categories.NewClientWithConfig(cfg)}
}

func (c *CategoryClient) PlanEnvironmentRefs() ([]string, error) {
	items, err := guardedExporterGet(func() ([]category.Category, error) {
		return c.client.GetCategoriesByScope(category.ScopePlanEnvironments)
	})
	if err != nil {
		return nil, wrapExporter(err)
	}
	ids := make([]string, constants.DefaultInitValue, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	return ids, nil
}
