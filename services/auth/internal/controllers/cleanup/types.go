package cleanup

import (
	"context"

	"github.com/telark/auth/internal/config"
	"github.com/telark/data/logger"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/response"
)

type (
	ListFn            func(ctx context.Context) ([]*resourcesshared.CleanupView, error)
	PatchFn           func(ctx context.Context, id string, body map[string]any) *response.GenericResponse
	AddFinalizerFn    func(ctx context.Context, id, finalizer string) *response.GenericResponse
	RemoveFinalizerFn func(ctx context.Context, id, finalizer string) *response.GenericResponse
	BackRef           struct {
		ResourceType string
		ArrayField   string
		List         ListFn
		Patch        PatchFn
	}
	Target struct {
		ResourceType    string
		Finalizer       string
		BackRefs        []BackRef
		RemoveFinalizer RemoveFinalizerFn
	}
	Reconciler struct {
		cfg     config.CleanupConfig
		targets map[string]Target
		lg      *logger.CustomLogger
	}
)
