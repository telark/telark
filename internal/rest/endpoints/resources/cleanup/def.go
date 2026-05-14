package cleanup

import "github.com/plsyro/rest/base"

const (
	AddFinalizer       base.Endpoint = "resources/{type}/{id}/finalizers/add"
	RemoveFinalizer    base.Endpoint = "resources/{type}/{id}/finalizers/remove"
	GetCleanupViewByID base.Endpoint = "resources/{type}/{id}/cleanup-view/get"
	ListCleanupViews   base.Endpoint = "resources/{type}/cleanup-view/get"
)
