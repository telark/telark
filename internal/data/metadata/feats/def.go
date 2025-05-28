package feats

import (
	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/metadata/base"
)

var MaintenanceAsFeatureMetadata = base.Metadata{
	BaseGroup: string(base.FEATS),
	Kind:      "MaintenanceAsFeature",
	Version:   string(base.ALPHA_1),
	Plural:    "maintenanceasfeatures",
	Namespace: common.BaseNamespace,
}
