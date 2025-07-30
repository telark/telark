package utils

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
)

func FormatCPU(milliValue int64) string {
	if milliValue < constants.CPU_MILLICORE_THRESHOLD {
		return fmt.Sprintf(constants.CPU_MILLICORE_FORMAT, milliValue)
	}
	return fmt.Sprintf(constants.CPU_CORE_FORMAT, float64(milliValue)/constants.CPU_CORE_DIVISOR)
}

func FormatMemory(bytes int64) string {
	switch {
	case bytes >= constants.GB:
		return formatMemoryUnit(bytes, constants.GB, constants.MEMORY_UNIT_GI)
	case bytes >= constants.MB:
		return formatMemoryUnit(bytes, constants.MB, constants.MEMORY_UNIT_MI)
	case bytes >= constants.KB:
		return formatMemoryUnit(bytes, constants.KB, constants.MEMORY_UNIT_KI)
	default:
		return formatMemoryUnit(bytes, 1, constants.MEMORY_UNIT_B)
	}
}

func formatMemoryUnit(bytes, unit int64, suffix string) string {
	if unit == 1 {
		return fmt.Sprintf(constants.MEMORY_BYTES_FORMAT, bytes)
	}
	return fmt.Sprintf(constants.MEMORY_UNIT_FORMAT, float64(bytes)/float64(unit), suffix)
}
