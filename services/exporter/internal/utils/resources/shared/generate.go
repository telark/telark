package shared

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/constants"
	"github.com/telark/kcore/crds/api"
)

func GenerateUniqueResourceID(
	md metadata.Metadata,
	config constants.IDConfig,
) (string, error) {
	for range constants.MaxUserIDGenerationAttempts {
		id, err := generateResourceID(config)
		if err != nil {
			continue
		}

		// Check if ID already exists
		exists, err := api.CheckCustomResourceExistsByName(id, md)
		if err != nil {
			continue
		}
		if !exists {
			return id, nil
		}
	}

	return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedToGenerateID), constants.MaxUserIDGenerationAttempts)
}

func generateResourceID(config constants.IDConfig) (string, error) {
	uuidValue := uuid.New()
	uuidBytes := uuidValue[:]
	hexStr := hex.EncodeToString(uuidBytes)
	totalHexChars := constants.DefaultInitValue
	for _, segment := range config.HexSegments {
		totalHexChars += segment
	}

	// UUID provides 32 hex chars, which is more than enough for our patterns (13 chars needed)
	if len(hexStr) < totalHexChars {
		return constants.EmptyString, errors.New(string(constants.ErrInsufficientHexCharacters))
	}

	// Build the ID with dashes using the specified segments
	var builder strings.Builder
	builder.Grow(len(config.Prefix) + totalHexChars + len(config.HexSegments))
	_, _ = builder.WriteString(config.Prefix)
	offset := constants.DefaultInitValue
	for i, segmentLen := range config.HexSegments {
		if i > constants.DefaultInitValue {
			_, _ = builder.WriteString("-")
		}
		if offset+segmentLen > len(hexStr) {
			return constants.EmptyString, errors.New(string(constants.ErrInsufficientHexCharacters))
		}
		_, _ = builder.WriteString(hexStr[offset : offset+segmentLen])
		offset += segmentLen
	}

	return builder.String(), nil
}
