package coreapps

import (
	"context"
	"testing"

	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/core"
	"github.com/telark/telark/services/discovery/internal/discovery/prewarm"
)

// With no derivation inputs there are no applications to assemble, so the
// response is well-formed and empty — this walks the assembly entry point
// without reaching Kubernetes.
func TestGetApplicationsEmpty(t *testing.T) {
	resp := core.GetApplications(context.Background(), nil, nil, prewarm.BuildPrewarmApplicationOptions())
	data, ok := resp.Data.(appresource.ResponseData)
	if !ok {
		t.Fatalf("response data has unexpected type %T", resp.Data)
	}
	if len(data.Applications) != constants.DefaultInitValue {
		t.Fatalf("expected no applications, got %d", len(data.Applications))
	}
}
