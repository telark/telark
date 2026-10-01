package coreapps

import (
	"context"
	"testing"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/core"
)

// With no derivation inputs there are no applications to assemble, so the
// response is well-formed and empty — this walks the assembly entry point
// without reaching Kubernetes.
func TestGetApplicationsEmpty(t *testing.T) {
	// No options: nothing is published or snapshotted, and the prewarm ones dial NATS for up to 30 s.
	resp := core.GetApplications(context.Background(), nil, nil, core.GetApplicationsOptions{})
	data, ok := resp.Data.(appresource.ResponseData)
	if !ok {
		t.Fatalf("response data has unexpected type %T", resp.Data)
	}
	if len(data.Applications) != constants.DefaultInitValue {
		t.Fatalf("expected no applications, got %d", len(data.Applications))
	}
}
