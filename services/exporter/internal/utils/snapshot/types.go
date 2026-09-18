package snapshot

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/telark/exporter/internal/constants"
)

var (
	lg                = constants.GetLogger(constants.PrefixMain)
	ErrLatestNotFound = errors.New(string(constants.ErrSnapshotNotFound))
	storageStats      storageStatsCache
)

// walk is held for the whole volume walk so at most one runs per process; mu
// only guards the published fields, so readers never wait on a walk.
type storageStatsCache struct {
	walk      sync.Mutex
	mu        sync.Mutex
	walks     atomic.Uint64
	usedBytes uint64
	count     int
	updatedAt int64
}

type SnapshotTarget struct {
	Path       string
	Namespace  string
	Generation int
}

type retentionFile struct {
	path string
	ver  int
}

const (
	baseSeparatorShift    = 1
	latestGenerationValue = constants.SnapshotLatestGenerationValue
	generationRequiredMsg = string(constants.ErrSnapshotGenerationRequired)
)

const (
	usedPercentScale = 100
	usedPercentCap   = 100
)

var manifestApplyOrder = map[string]int{
	constants.KindServiceAccount:          constants.ManifestOrderServiceAccount,
	constants.KindConfigMap:               constants.ManifestOrderConfigMap,
	constants.KindSecret:                  constants.ManifestOrderSecret,
	constants.KindPersistentVolumeClaim:   constants.ManifestOrderPersistentVolumeClaim,
	constants.KindService:                 constants.ManifestOrderService,
	constants.KindNetworkPolicy:           constants.ManifestOrderNetworkPolicy,
	constants.KindDeployment:              constants.ManifestOrderDeployment,
	constants.KindStatefulSet:             constants.ManifestOrderStatefulSet,
	constants.KindDaemonSet:               constants.ManifestOrderDaemonSet,
	constants.KindJob:                     constants.ManifestOrderJob,
	constants.KindCronJob:                 constants.ManifestOrderCronJob,
	constants.KindIngress:                 constants.ManifestOrderIngress,
	constants.KindHorizontalPodAutoscaler: constants.ManifestOrderHorizontalPodAutoscaler,
	constants.KindVerticalPodAutoscaler:   constants.ManifestOrderVerticalPodAutoscaler,
}
