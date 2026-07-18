package status

import (
	"github.com/telark/auth/internal/constants"
	statuseps "github.com/telark/rest/endpoints/status"
	statushandler "github.com/telark/rest/handlers/status"
)

// One handler serves all three probes: health and liveness always succeed, and
// readiness runs the check. auth has no readiness gate, so it is always ready.
var ProbeHandler = statushandler.NewProbeHandler(
	string(statuseps.ReadinessCheck),
	nil,
	statushandler.Messages{
		Ready:    constants.HealthMessageHealthy,
		NotReady: constants.HealthMessageNotReady,
	},
)
