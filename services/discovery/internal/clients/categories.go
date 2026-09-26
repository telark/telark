package clients

import (
	"github.com/telark/data/classification/category"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	categoryeps "github.com/telark/rest/endpoints/classification/category"
)

// No rest client covers categories yet, so this reads the exporter route through the shared client.
type CategoryClient struct {
	client *shared.Client
}

func NewCategoryClient() *CategoryClient {
	cfg := &shared.ClientConfig{Timeout: constants.CategoryReadTimeout}
	return &CategoryClient{client: shared.NewWithConfig(base.Exporter, cfg)}
}

func (c *CategoryClient) PlanEnvironmentIDs() ([]string, error) {
	scoped := c.client.WithParams(map[string]string{constants.CategoryScopeParam: category.ScopePlanEnvironments})
	items, err := guardedExporterGet(func() ([]category.Category, error) {
		return shared.GetListTyped[category.Category](scoped, categoryeps.GetCategoriesByScope)
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
