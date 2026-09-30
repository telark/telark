package corehealth

import (
	"testing"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// With no workload resources there is nothing to poll, so health resolves to the
// unknown/zero state rather than reaching for a cluster.
func TestComputeHealthNoWorkloads(t *testing.T) {
	app := &appresource.Application{
		Resources: []appresource.Resource{
			{Namespace: "prod", Kind: "Service", Name: "svc"},
			{Namespace: "prod", Kind: "ConfigMap", Name: "cfg"},
		},
	}
	h := core.ComputeHealth(app)
	testutil.Equal(t, "status", h.Status, "unknown")
	testutil.Equal(t, "ready", h.ReadyReplicas, constants.DefaultInitValue)
	testutil.Equal(t, "total", h.TotalReplicas, constants.DefaultInitValue)
}

// An application with no resources at all is also unknown.
func TestComputeHealthEmpty(t *testing.T) {
	testutil.Equal(t, "status", core.ComputeHealth(&appresource.Application{}).Status, "unknown")
}
