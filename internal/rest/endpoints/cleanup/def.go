package cleanup

import "github.com/telark/telark/internal/rest/base"

const (
	AddFinalizer       base.Endpoint = "cleanup/{type}/{id}/finalizer"
	RemoveFinalizer    base.Endpoint = "cleanup/{type}/{id}/finalizer"
	GetCleanupViewByID base.Endpoint = "cleanup/{type}/{id}"
	ListCleanupViews   base.Endpoint = "cleanup/{type}"
)
