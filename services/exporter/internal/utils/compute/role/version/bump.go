package version

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/telark/exporter/internal/constants"
)

type ChangeType int

const (
	ChangeTypePatch ChangeType = iota
	ChangeTypeMinor
	ChangeTypeMajor
)

func Bump(currentVersion string, changeType ChangeType) string {
	major, minor, patch := parseVersion(currentVersion)

	switch changeType {
	case ChangeTypeMajor:
		major++
		minor = constants.DefaultInitValue
		patch = constants.DefaultInitValue
	case ChangeTypeMinor:
		minor++
		patch = constants.DefaultInitValue
	case ChangeTypePatch:
		patch++
	default:
		return currentVersion
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch)
}

func Initialize() string {
	return "v1.0.0"
}

func parseVersion(version string) (major, minor, patch int) {
	version = strings.TrimSpace(version)
	if version == constants.EmptyString {
		return constants.DefaultIncrementValue, constants.DefaultInitValue, constants.DefaultInitValue
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) == constants.DefaultInitValue {
		return constants.DefaultIncrementValue, constants.DefaultInitValue, constants.DefaultInitValue
	}

	if len(parts) > constants.DefaultInitValue {
		if m, err := strconv.Atoi(parts[0]); err == nil {
			major = m
		} else {
			major = constants.DefaultIncrementValue
		}
	}

	if len(parts) > constants.DefaultIncrementValue {
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minor = m
		}
	}

	if len(parts) > constants.DefaultMajorPart {
		if p, err := strconv.Atoi(parts[constants.DefaultMajorPart]); err == nil {
			patch = p
		}
	}

	return major, minor, patch
}
