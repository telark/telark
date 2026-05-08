package plans

import (
	"github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var ProtectionPlanMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "ProtectionPlan",
	Version:   string(base.Alpha1),
	Plural:    "protectionplans",
	Namespace: globalshared.BaseNamespace,
}
