package authrequests

import (
	"context"
	"fmt"
	"time"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// A run takes minutes, so a request past the retention is history: the cluster keeps it that long beside the audit log.
func StartCleanup(ctx context.Context) {
	sweep(time.Now())
	ticker := time.NewTicker(constants.AuthRequestCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sweep(now)
		}
	}
}

func sweep(now time.Time) {
	items, err := list()
	if err != nil {
		lg.Warn(err.Error())
		return
	}
	retention := envmanager.GetAuthRequestRetention()
	deleted := constants.DefaultInitValue
	for i := range items {
		if now.Sub(items[i].GetCreationTimestamp().Time) < retention {
			continue
		}
		name := items[i].GetName()
		// Another replica's sweep may have deleted it first.
		if result := api.DeleteCustomResourceByName(name, metadata.AuthRequestMetadata); result.Error != nil &&
			!k8serrors.IsNotFound(result.Error) {
			lg.Warn(fmt.Sprintf(string(constants.ErrAuthRequestDeleteFailed), name, result.Error))
			continue
		}
		deleted++
	}
	if deleted > constants.DefaultInitValue {
		lg.Info(fmt.Sprintf(string(constants.InfAuthRequestsCleaned), deleted, int(retention/constants.AuthRequestDay)))
	}
}
