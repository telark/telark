package status

import (
	"net/http"

	"github.com/telark/exporter/internal/constants"
	kcoreconst "github.com/telark/kcore/constants"
	"github.com/telark/kcore/health"
	statuseps "github.com/telark/rest/endpoints/status"
	statushandler "github.com/telark/rest/handlers/status"
)

var ProbeHandler = statushandler.NewProbeHandler(
	string(statuseps.ReadinessCheck),
	readinessOK,
	statushandler.Messages{
		Ready:    string(constants.InfSuccessStatusMessage),
		NotReady: constants.UnknownValue,
	},
)

func readinessOK(r *http.Request) bool {
	return health.IsK8sReachable(r.Context(), kcoreconst.ZeroValue, kcoreconst.ZeroValue)
}
