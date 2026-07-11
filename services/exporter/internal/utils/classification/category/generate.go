package category

import (
	metadata "github.com/telark/data/metadata/classification"
	"github.com/telark/exporter/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
)

func GenerateUniqueCategoryID() (string, error) {
	return resourcesshared.GenerateUniqueResourceID(
		metadata.CategoryAsClassificationMetadata,
		constants.CategoryIDConfig,
	)
}
