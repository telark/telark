package plans

import (
	"github.com/telark/data/metadata/base"
	globalshared "github.com/telark/data/shared"
)

var ProtectionPlanMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "ProtectionPlan",
	Version:   string(base.Alpha1),
	Plural:    "protectionplans",
	Namespace: globalshared.BaseNamespace,
}
