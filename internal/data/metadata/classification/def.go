package classification

import (
	metadata "github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var CategoryAsClassificationMetadata = metadata.Metadata{
	BaseGroup: string(metadata.Classification),
	Kind:      "CategoryAsClassification",
	Version:   string(metadata.Alpha1),
	Plural:    "categoriesasclassifications",
	Namespace: globalshared.BaseNamespace,
}

