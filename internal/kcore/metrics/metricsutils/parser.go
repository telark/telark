package metricsutils

import (
	"strconv"
	"strings"

	"github.com/plsyro/kcore-pkg/constants"
)

func ParseCPU(cpuStr string) int64 {
	if cpuStr == "" || cpuStr == constants.NAValue {
		return 0
	}

	if strings.HasSuffix(cpuStr, constants.CPUUnitMillicore) {
		value := strings.TrimSuffix(cpuStr, constants.CPUUnitMillicore)
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

func ParseMemory(memoryStr string) int64 {
	if memoryStr == "" || memoryStr == constants.NAValue {
		return 0
	}

	lowerStr := strings.ToLower(memoryStr)

	if value := parseMemoryWithUnit(lowerStr, "gi", 1024*1024*1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, "mi", 1024*1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, "ki", 1024); value > 0 {
		return value
	}

	if value := parseMemoryWithUnit(lowerStr, constants.MemoryUnitKB, 1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MemoryUnitMB, 1024*1024); value > 0 {
		return value
	}
	if value := parseMemoryWithUnit(lowerStr, constants.MemoryUnitGB, 1024*1024*1024); value > 0 {
		return value
	}

	if val, err := strconv.ParseInt(memoryStr, 10, 64); err == nil {
		return val
	}

	return 0
}

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
