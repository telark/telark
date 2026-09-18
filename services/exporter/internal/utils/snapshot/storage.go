package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/kcore/resources/core"
)

type SnapshotStorageMetrics struct {
	Bytes   uint64
	KB      float64
	MB      float64
	Percent float64
}

type SnapshotInfos struct {
	TotalPVCSpace  SnapshotStorageMetrics
	ConsumedSpace  SnapshotStorageMetrics
	AvailableSpace SnapshotStorageMetrics
	TotalSnapshots int
	UpdatedAt      int64
}

func FormatBytes(bytes uint64) string {
	const (
		kilo = 1024
		mega = kilo * 1024
		giga = mega * 1024
	)

	switch {
	case bytes < kilo:
		return fmt.Sprintf("%d B", bytes)
	case bytes < mega:
		return fmt.Sprintf("%.2f KB", float64(bytes)/kilo)
	case bytes < giga:
		return fmt.Sprintf("%.2f MB", float64(bytes)/mega)
	default:
		return fmt.Sprintf("%.2f GB", float64(bytes)/giga)
	}
}

func ComputeSnapshotsStorageStats(snapshotsPath string) (uint64, int, error) {
	var total uint64
	count := constants.DefaultInitValue
	walkErr := filepath.Walk(snapshotsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		size := info.Size()
		if size < int64(constants.DefaultInitValue) {
			return nil
		}
		total += uint64(size)
		if _, ok := parseGenerationFilename(filepath.Base(path)); ok {
			count++
		}
		return nil
	})
	return total, count, walkErr
}

func walkStorageStats(snapshotsPath string) (usedBytes uint64, count int) {
	usedBytes, count, err := ComputeSnapshotsStorageStats(snapshotsPath)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotPVCStatFailed), snapshotsPath, err))
		return uint64(constants.DefaultInitValue), constants.DefaultInitValue
	}
	return usedBytes, count
}

// A walk of the volume can take longer than the request timeout, so requests
// read the last published value and only the refresher (or a first-request
// kick) walks. TryLock is the singleflight: a tick or kick that lands during a
// walk is dropped, never queued.
func publishStorageStats(snapshotsPath string) {
	if !storageStats.walk.TryLock() {
		return
	}
	started := time.Now()
	usedBytes, count := walkStorageStats(snapshotsPath)
	storageStats.mu.Lock()
	storageStats.usedBytes = usedBytes
	storageStats.count = count
	storageStats.updatedAt = time.Now().Unix()
	storageStats.mu.Unlock()
	storageStats.walk.Unlock()
	storageStats.walks.Add(constants.DefaultIncrementValue)
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotStatsRefreshed), count, usedBytes, time.Since(started)))
}

func RefreshStorageStats() {
	publishStorageStats(envmanager.GetSnapshotsPath())
}

// Completed walks; the counter is incremented only once the result is published.
func StorageStatsWalks() uint64 {
	return storageStats.walks.Load()
}

func StartStorageStatsRefresher(ctx context.Context) {
	interval := envmanager.GetSnapshotStatsRefreshInterval()
	if interval <= constants.DefaultInitValue {
		lg.Info(string(constants.InfSnapshotStatsRefreshDisabled))
		return
	}
	RefreshStorageStats()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			RefreshStorageStats()
		}
	}
}

// Before the first walk has published, callers get zeros with UpdatedAt 0 and
// the walk is kicked off in the background rather than awaited.
func currentStorageStats() (usedBytes uint64, count int, updatedAt int64) {
	if envmanager.GetSnapshotStatsRefreshInterval() <= constants.DefaultInitValue {
		usedBytes, count = walkStorageStats(envmanager.GetSnapshotsPath())
		return usedBytes, count, time.Now().Unix()
	}
	storageStats.mu.Lock()
	defer storageStats.mu.Unlock()
	if storageStats.updatedAt == constants.DefaultInitValue {
		go publishStorageStats(envmanager.GetSnapshotsPath())
	}
	return storageStats.usedBytes, storageStats.count, storageStats.updatedAt
}

func BuildSnapshotInfos() SnapshotInfos {
	usedBytes, snapshotCount, updatedAt := currentStorageStats()
	totalBytes := snapshotPVCTotalBytes()
	availableBytes := snapshotAvailableBytes(totalBytes, usedBytes)
	consumedPercent := snapshotPercent(usedBytes, totalBytes)
	availablePercent := snapshotPercent(availableBytes, totalBytes)
	return SnapshotInfos{
		TotalPVCSpace:  snapshotStorageMetrics(totalBytes, constants.DefaultInitValue),
		ConsumedSpace:  snapshotStorageMetrics(usedBytes, consumedPercent),
		AvailableSpace: snapshotStorageMetrics(availableBytes, availablePercent),
		TotalSnapshots: snapshotCount,
		UpdatedAt:      updatedAt,
	}
}

func snapshotPVCTotalBytes() uint64 {
	pvcNamespace := envmanager.GetSnapshotsPVCNamespace()
	pvcName := envmanager.GetSnapshotsPVCName()
	pvcTotalBytes, err := core.GetPersistentVolumeClaimCapacityBytes(pvcNamespace, pvcName)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotPVCGetFailed), pvcName, pvcNamespace, err))
		return uint64(constants.DefaultInitValue)
	}
	if pvcTotalBytes < int64(constants.DefaultInitValue) {
		return uint64(constants.DefaultInitValue)
	}
	return uint64(pvcTotalBytes)
}

func snapshotAvailableBytes(totalBytes uint64, usedBytes uint64) uint64 {
	if usedBytes >= totalBytes {
		return uint64(constants.DefaultInitValue)
	}
	return totalBytes - usedBytes
}

func snapshotStorageMetrics(bytes uint64, percent float64) SnapshotStorageMetrics {
	kb := float64(bytes) / constants.SnapshotBytesPerKilobyte
	mb := kb / constants.SnapshotKilobytesPerMegabyte
	return SnapshotStorageMetrics{
		Bytes:   bytes,
		KB:      kb,
		MB:      mb,
		Percent: percent,
	}
}

func snapshotPercent(part uint64, total uint64) float64 {
	if total == uint64(constants.DefaultInitValue) {
		return float64(constants.DefaultInitValue)
	}
	return (float64(part) * constants.SnapshotPercentScale) / float64(total)
}

func GetStorageInfo() (pvcAvailable string, pvcTotal string, pvcUsedPercent string) {
	pvcAvailable = constants.UnknownValue
	pvcTotal = constants.UnknownValue
	pvcUsedPercent = constants.UnknownValue

	pvcNamespace := envmanager.GetSnapshotsPVCNamespace()
	pvcName := envmanager.GetSnapshotsPVCName()

	pvcTotalBytes, err := core.GetPersistentVolumeClaimCapacityBytes(pvcNamespace, pvcName)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotPVCGetFailed), pvcName, pvcNamespace, err))
		return pvcAvailable, pvcTotal, pvcUsedPercent
	}
	if pvcTotalBytes < int64(constants.DefaultInitValue) {
		return pvcAvailable, pvcTotal, pvcUsedPercent
	}
	pvcTotal = FormatBytes(uint64(pvcTotalBytes))

	usedBytes, _, _ := currentStorageStats()
	pvcAvailable = FormatBytes(snapshotAvailableBytes(uint64(pvcTotalBytes), usedBytes))

	var usedPercent uint64
	if pvcTotalBytes > int64(constants.DefaultInitValue) {
		usedPercent = min(
			(usedBytes*usedPercentScale)/uint64(pvcTotalBytes),
			uint64(usedPercentCap),
		)
	}
	pvcUsedPercent = fmt.Sprintf("%d%%", usedPercent)

	return pvcAvailable, pvcTotal, pvcUsedPercent
}
