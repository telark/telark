package feats

import (
	metadata "github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var MaintenanceAsFeatureMetadata = metadata.Metadata{
	BaseGroup: string(metadata.Feats),
	Kind:      "MaintenanceAsFeature",
	Version:   string(metadata.Alpha1),
	Plural:    "maintenanceasfeatures",
	Namespace: globalshared.BaseNamespace,
}
