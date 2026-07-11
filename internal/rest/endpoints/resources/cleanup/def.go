package cleanup

import "github.com/telark/rest/base"

const (
	AddFinalizer       base.Endpoint = "cleanup/resources/{type}/{id}/finalizers/add"
	RemoveFinalizer    base.Endpoint = "cleanup/resources/{type}/{id}/finalizers/remove"
	GetCleanupViewByID base.Endpoint = "cleanup/resources/{type}/{id}/view/get"
	ListCleanupViews   base.Endpoint = "cleanup/resources/{type}/views/get"
)
