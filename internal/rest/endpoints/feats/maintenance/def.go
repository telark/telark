package maintenance

import (
	"github.com/plsyro/rest-pkg/base"
)

const (
	// Custom Endpoints
	ENABLE_MAINTENANCE  base.Endpoint = "feats/maintenance/enable"
	SUSPEND_MAINTENANCE base.Endpoint = "feats/maintenance/suspend"
	UPDATE_MAINTENANCE  base.Endpoint = "feats/maintenance/update"
	REMOVE_MAINTENANCE  base.Endpoint = "feats/maintenance/remove"

	// ******************Used By Exporter Service******************
	// Create Endpoints
	CREATE_MAINTENANCE base.Endpoint = "feats/maintenance/create"

	// Get Endpoints
	GET_MAINTENANCE base.Endpoint = "feats/maintenance/{name}/get"

	// Patch Endpoints
	PATCH_MAINTENANCE base.Endpoint = "feats/maintenance/{name}/patch"

	// Delete Endpoints
	DELETE_MAINTENANCE base.Endpoint = "feats/maintenance/{name}/delete"
)
