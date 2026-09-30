package category

import (
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/services/exporter/internal/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
)

func GenerateUniqueCategoryID() (string, error) {
	return resourcesshared.GenerateUniqueResourceID(
		metadata.CategoryMetadata,
		constants.CategoryIDConfig,
	)
}
