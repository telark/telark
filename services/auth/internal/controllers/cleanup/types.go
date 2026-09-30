package cleanup

import (
	"context"

	"github.com/telark/telark/internal/data/logger"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/auth/internal/config"
)

type (
	ListFn            func(ctx context.Context) ([]*resourcesshared.CleanupView, error)
	PatchFn           func(ctx context.Context, id string, body map[string]any) *response.GenericResponse
	AddFinalizerFn    func(ctx context.Context, id, finalizer string) *response.GenericResponse
	RemoveFinalizerFn func(ctx context.Context, id, finalizer string) *response.GenericResponse
	// Owned records that die with the target, run before the back-references so
	// access is revoked first; nil when the target owns nothing.
	PurgeFn func(id string) error
	BackRef struct {
		ResourceType string
		ArrayField   string
		List         ListFn
		Patch        PatchFn
	}
	Target struct {
		ResourceType    string
		Finalizer       string
		Purge           PurgeFn
		BackRefs        []BackRef
		RemoveFinalizer RemoveFinalizerFn
	}
	Reconciler struct {
		cfg     config.CleanupConfig
		targets map[string]Target
		lg      *logger.CustomLogger
	}
)
