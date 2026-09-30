package categories

import "github.com/telark/telark/internal/rest/base"

const (
	CreateCategory     base.Endpoint = "categories"
	GetAllCategories   base.Endpoint = "categories"
	GetCategoryByID    base.Endpoint = "categories/{id}"
	PatchCategoryByID  base.Endpoint = "categories/{id}"
	DeleteCategoryByID base.Endpoint = "categories/{id}"

	QueryScope = "scope"
)
