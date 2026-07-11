package classification

import (
	metadata "github.com/telark/data/metadata/base"
	globalshared "github.com/telark/data/shared"
)

var CategoryAsClassificationMetadata = metadata.Metadata{
	BaseGroup: string(metadata.Classification),
	Kind:      "CategoryAsClassification",
	Version:   string(metadata.Alpha1),
	Plural:    "categoriesasclassifications",
	Namespace: globalshared.BaseNamespace,
}
