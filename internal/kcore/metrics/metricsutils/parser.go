package metricsutils

import (
	"strconv"
	"strings"

	"github.com/plsyro/kcore/constants"
)

func ParseCPU(cpuStr string) int64 {
	if cpuStr == "" || cpuStr == constants.NAValue {
		return constants.EmptySliceLength
	}

	if strings.HasSuffix(cpuStr, constants.CPUUnitMillicore) {
		value := strings.TrimSuffix(cpuStr, constants.CPUUnitMillicore)
		if val, err := strconv.ParseInt(value, constants.Base10, constants.Base64); err == nil {
			return val
		}
	} else {
		if val, err := strconv.ParseFloat(cpuStr, constants.Base64); err == nil {
			return int64(val * constants.CPUMillicoreThreshold)
		}
	}
	return constants.EmptySliceLength
}

func ParseMemory(memoryStr string) int64 {
	if memoryStr == "" || memoryStr == constants.NAValue {
		return constants.EmptySliceLength
	}

	lowerStr := strings.ToLower(memoryStr)

	switch {
	case strings.HasSuffix(lowerStr, "gi"):
		return parseMemoryWithUnit(lowerStr, "gi", constants.GB)
	case strings.HasSuffix(lowerStr, "mi"):
		return parseMemoryWithUnit(lowerStr, "mi", constants.MB)
	case strings.HasSuffix(lowerStr, "ki"):
		return parseMemoryWithUnit(lowerStr, "ki", constants.KB)
	case strings.HasSuffix(lowerStr, "kb"):
		return parseMemoryWithUnit(lowerStr, "kb", constants.KB)
	case strings.HasSuffix(lowerStr, "mb"):
		return parseMemoryWithUnit(lowerStr, "mb", constants.MB)
	case strings.HasSuffix(lowerStr, "gb"):
		return parseMemoryWithUnit(lowerStr, "gb", constants.GB)
	default:
		if val, err := strconv.ParseInt(memoryStr, constants.Base10, constants.Base64); err == nil {
			return val
		}
		return constants.EmptySliceLength
	}
}

func parseMemoryWithUnit(memoryStr, unit string, multiplier int64) int64 {
	if !strings.HasSuffix(memoryStr, unit) {
		return constants.EmptySliceLength
	}

	value := strings.TrimSuffix(memoryStr, unit)
	if val, err := strconv.ParseFloat(value, constants.Base64); err == nil {
		return int64(val * float64(multiplier))
	}

	return constants.EmptySliceLength
}
