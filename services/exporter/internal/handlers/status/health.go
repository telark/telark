package status

import (
	"net/http"
	"strings"

	"github.com/telark/exporter/internal/constants"
	kcoreconst "github.com/telark/kcore/constants"
	"github.com/telark/kcore/health"
	statuseps "github.com/telark/rest/endpoints/status"
	"github.com/telark/rest/response"
)

func ProbeHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, string(statuseps.ReadinessCheck)) && !readinessOK(r) {
		response.SendSingleResponse(
			w,
			response.NewGenericResponse(
				http.StatusServiceUnavailable,
				response.OperationUnprocessed,
				nil,
				constants.UnknownValue,
			),
		)
		return
	}

	response.SendSingleResponse(
		w,
		response.NewGenericResponse(
			http.StatusOK,
			response.OperationSuccess,
			nil,
			string(constants.InfSuccessStatusMessage),
		),
	)
}

func readinessOK(r *http.Request) bool {
	return health.IsK8sReachable(r.Context(), kcoreconst.ZeroValue, kcoreconst.ZeroValue)
}
