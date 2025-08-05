package feats

import (
	"github.com/plsyro/data/metadata/base"
	globalShared "github.com/plsyro/data/shared"
)

var MaintenanceAsFeatureMetadata = base.Metadata{
	BaseGroup: string(base.Feats),
	Kind:      "MaintenanceAsFeature",
	Version:   string(base.Alpha1),
	Plural:    "maintenanceasfeatures",
	Namespace: globalShared.BaseNamespace,
}
