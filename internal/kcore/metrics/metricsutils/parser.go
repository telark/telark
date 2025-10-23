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

	if value, ok := strings.CutSuffix(cpuStr, constants.CPUUnitMillicore); ok {
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
	value, ok := strings.CutSuffix(memoryStr, unit)
	if !ok {
		return constants.EmptySliceLength
	}
	if val, err := strconv.ParseFloat(value, constants.Base64); err == nil {
		return int64(val * float64(multiplier))
	}
	return constants.EmptySliceLength
}
