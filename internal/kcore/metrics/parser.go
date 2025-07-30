package metrics

import (
	"strconv"
	"strings"

	"github.com/plsyro/kcore-pkg/constants"
)

// ParseCPU parses CPU string to millicores
func ParseCPU(cpuStr string) int64 {
	if cpuStr == "" || cpuStr == constants.NA_VALUE {
		return 0
	}

	if strings.HasSuffix(cpuStr, constants.CPU_UNIT_MILLICORE) {
		value := strings.TrimSuffix(cpuStr, constants.CPU_UNIT_MILLICORE)
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			return val
		}
	} else {
		if val, err := strconv.ParseFloat(cpuStr, 64); err == nil {
			return int64(val * 1000)
		}
	}
	return 0
}

// ParseMemory parses memory string to bytes
func ParseMemory(memoryStr string) int64 {
	if memoryStr == "" || memoryStr == constants.NA_VALUE {
		return 0
	}

	lowerStr := strings.ToLower(memoryStr)

	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_KB, 1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_MB, 1024*1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_GB, 1024*1024*1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_KB, 1000); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_MB, 1000*1000); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MEMORY_UNIT_GB, 1000*1000*1000); value > 0 {
		return value
	}

	if val, err := strconv.ParseInt(memoryStr, 10, 64); err == nil {
		return val
	}

	return 0
}

// parseMemoryWithUnit parses memory with specific unit
func parseMemoryWithUnit(memoryStr, unit string, multiplier int64) int64 {
	if !strings.HasSuffix(memoryStr, unit) {
		return 0
	}

	value := strings.TrimSuffix(memoryStr, unit)
	if val, err := strconv.ParseFloat(value, 64); err == nil {
		return int64(val * float64(multiplier))
	}

	return 0
}
