package status

import (
	"net/http"
	"time"

	"github.com/telark/notifier/internal/constants"
	"github.com/telark/rest/base"
	statuseps "github.com/telark/rest/endpoints/status"
	statushandler "github.com/telark/rest/handlers/status"
	"github.com/telark/rest/router"
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
