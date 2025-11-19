package category

import "github.com/plsyro/rest/base"

const (
	CreateCategory       base.Endpoint = "classification/categories/create"
	GetAllCategories     base.Endpoint = "classification/categories/get"
	GetCategoryByID      base.Endpoint = "classification/categories/{id}/get"
	GetCategoriesByScope base.Endpoint = "classification/categories/scope/{scope}/get"
	PatchCategoryByID    base.Endpoint = "classification/categories/{id}/patch"
	DeleteCategoryByID   base.Endpoint = "classification/categories/{id}/delete"
)
