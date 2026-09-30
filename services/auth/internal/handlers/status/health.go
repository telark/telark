package status

import (
	statuseps "github.com/telark/telark/internal/rest/endpoints/status"
	statushandler "github.com/telark/telark/internal/rest/handlers/status"
	"github.com/telark/telark/services/auth/internal/constants"
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
