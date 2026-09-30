package reports

import (
	"context"
	"fmt"
	"net/http"
	"time"

	plansmd "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	"github.com/telark/telark/services/exporter/internal/utils/artifact"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Shares the snapshot GC interval knob: SNAPSHOT_GC_INTERVAL_SEC <= 0 disables
// both sweeps. Its own goroutine so a reports panic never stops the snapshot sweep.
func StartReportsGC(ctx context.Context) {
	interval := envmanager.GetSnapshotGCInterval()
	if interval <= constants.DefaultInitValue {
		lg.Info(string(constants.InfReportsGCDisabled))
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if artifact.TickAllowed(ctx, exprdb.Get(), constants.ReportsGCLockKey, interval/constants.SnapshotGCLockTTLDivisor) {
				RunGuardedWith(RunReportsGC)
			}
		}
	}
}

func RunGuardedWith(sweep func()) {
	defer func() {
		if r := recover(); r != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnReportsGCPanic), r))
		}
	}()
	sweep()
}

// The CR name is the plan id, so no spec decode is needed.
func RunReportsGC() {
	result := api.ListCustomResources(plansmd.ProtectionPlanMetadata)
	if result.Status != http.StatusOK || result.Error != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnReportsSweepListFailed), result.Error))
		return
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok || list == nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnReportsSweepListFailed), result.Data))
		return
	}
	live := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		live[list.Items[i].GetName()] = struct{}{}
	}
	SweepWith(live)
}

// Never sweeps on an empty set: with no live plans every directory would look
// unreferenced.
func SweepWith(live map[string]struct{}) {
	if len(live) == constants.DefaultInitValue {
		return
	}
	removed, temps, scanned := reportsutil.SweepOrphans(envmanager.GetReportsPath(), live, constants.ReportsSweepMinAge)
	lg.Info(fmt.Sprintf(string(constants.InfReportsSwept), removed, temps, scanned))
}
