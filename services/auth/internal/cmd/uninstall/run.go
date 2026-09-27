package uninstall

import (
	"context"
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	"github.com/telark/data/logger"
)

// Best effort: a failure must not block the uninstall, and the leader sweeper
// restores any finalizer removed here if the release is installed again.
func RemoveFinalizers(lg *logger.CustomLogger) {
	removed, failed := constants.DefaultInitValue, constants.DefaultInitValue
	for _, resourceType := range cleanupctrl.RegisteredResourceTypes() {
		ops, _ := cleanupctrl.GetResourceOps(resourceType)
		views, err := ops.List(context.Background())
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrRemoveFinalizersListFailed), resourceType, err))
			failed++
			continue
		}
		for _, v := range views {
			if !v.HasFinalizer(ops.Finalizer) {
				continue
			}
			resp := ops.RemoveFinalizer(context.Background(), v.Name, ops.Finalizer)
			if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
				status := constants.DefaultInitValue
				if resp != nil {
					status = resp.Status
				}
				lg.Error(fmt.Sprintf(string(constants.ErrCleanupRemoveFinalizerFail), resourceType, v.Name, status))
				failed++
				continue
			}
			removed++
		}
	}
	lg.Info(fmt.Sprintf(string(constants.LogFinalizersRemoved), removed, failed))
}
