package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/telark/exporter/constants"
	envmanager "github.com/telark/exporter/managers/envs"
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

func ComputeSnapshotsUsedBytes(snapshotsPath string) (uint64, error) {
	var total uint64
	walkErr := filepath.Walk(snapshotsPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		sz := info.Size()
		if sz < int64(constants.DefaultInitValue) {
			return nil
		}
		total += uint64(sz)

		return nil
	})

	return total, walkErr
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

func BuildSnapshotInfos() SnapshotInfos {
	snapshotsPath := envmanager.GetSnapshotsPath()
	usedBytes, snapshotCount, walkErr := ComputeSnapshotsStorageStats(snapshotsPath)
	if walkErr != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotPVCStatFailed), snapshotsPath, walkErr))
		usedBytes = uint64(constants.DefaultInitValue)
		snapshotCount = constants.DefaultInitValue
	}
	totalBytes := snapshotPVCTotalBytes()
	availableBytes := snapshotAvailableBytes(totalBytes, usedBytes)
	consumedPercent := snapshotPercent(usedBytes, totalBytes)
	availablePercent := snapshotPercent(availableBytes, totalBytes)
	return SnapshotInfos{
		TotalPVCSpace:  snapshotStorageMetrics(totalBytes, constants.DefaultInitValue),
		ConsumedSpace:  snapshotStorageMetrics(usedBytes, consumedPercent),
		AvailableSpace: snapshotStorageMetrics(availableBytes, availablePercent),
		TotalSnapshots: snapshotCount,
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

	snapshotsPath := envmanager.GetSnapshotsPath()
	usedBytes, walkErr := ComputeSnapshotsUsedBytes(snapshotsPath)
	if walkErr != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotPVCStatFailed), snapshotsPath, walkErr))
		usedBytes = uint64(constants.DefaultInitValue)
	}

	availableBytes := pvcTotalBytes - int64(usedBytes)
	if availableBytes < int64(constants.DefaultInitValue) {
		availableBytes = int64(constants.DefaultInitValue)
	}
	pvcAvailable = FormatBytes(uint64(availableBytes))

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
