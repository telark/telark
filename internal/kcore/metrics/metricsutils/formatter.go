package metricsutils

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
)

func FormatCPU(milliValue int64) string {
	if milliValue < constants.CPUMillicoreThreshold {
		return fmt.Sprintf(constants.CPUMillicoreFormat, milliValue)
	}
	return fmt.Sprintf(constants.CPUCoreFormat, float64(milliValue)/constants.CPUCoreDivisor)
}

func FormatMemory(bytes int64) string {
	switch {
	case bytes >= constants.GB:
		return formatMemoryUnit(bytes, constants.GB, constants.MemoryUnitGi)
	case bytes >= constants.MB:
		return formatMemoryUnit(bytes, constants.MB, constants.MemoryUnitMi)
	case bytes >= constants.KB:
		return formatMemoryUnit(bytes, constants.KB, constants.MemoryUnitKi)
	default:
		return formatMemoryUnit(bytes, 1, constants.MemoryUnitB)
	}
}

func formatMemoryUnit(bytes, unit int64, suffix string) string {
	if unit == 1 {
		return fmt.Sprintf(constants.MemoryBytesFormat, bytes)
	}
	return fmt.Sprintf(constants.MemoryUnitFormat, float64(bytes)/float64(unit), suffix)
}
