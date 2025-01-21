package maintenance

import (
	"github.com/plsyro/rest-pkg/base"
)

const (
	// Create Endpoints
	CREATE_MAINTENANCE base.Endpoint = "feats/maintenance/create"

	// Custom Endpoints
	ENABLE_MAINTENANCE  base.Endpoint = "feats/maintenance/enable"
	DISABLE_MAINTENANCE base.Endpoint = "feats/maintenance/disable"
)
