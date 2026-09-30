package clients

import (
	"github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/rest/clients/categories"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/services/discovery/internal/constants"
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
