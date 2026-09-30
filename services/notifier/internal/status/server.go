package status

import (
	"net/http"
	"time"

	"github.com/telark/telark/internal/rest/base"
	statuseps "github.com/telark/telark/internal/rest/endpoints/status"
	statushandler "github.com/telark/telark/internal/rest/handlers/status"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/notifier/internal/constants"
)

func NewServer(connected func() bool) *http.Server {
	probe := statushandler.NewProbeHandler(
		string(statuseps.ReadinessCheck),
		func(_ *http.Request) bool { return connected() },
		statushandler.Messages{
			Ready:    string(constants.InfoStatusReady),
			NotReady: string(constants.InfoStatusNotReady),
		},
	)

	routes := []router.Route{
		router.CreateRoute(base.Get, statuseps.LivenessCheck, probe),
		router.CreateRoute(base.Get, statuseps.ReadinessCheck, probe),
	}

	return &http.Server{
		Addr:              constants.StatusServerPort,
		Handler:           router.NewRouter(routes),
		ReadHeaderTimeout: constants.StatusServerReadTimeoutSec * time.Second,
	}
}
