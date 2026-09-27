package category

import (
	metadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
)

func GenerateUniqueCategoryID() (string, error) {
	return resourcesshared.GenerateUniqueResourceID(
		metadata.CategoryMetadata,
		constants.CategoryIDConfig,
	)
}
