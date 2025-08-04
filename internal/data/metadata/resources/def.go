package resources

import (
	"github.com/plsyro/data-pkg/metadata/base"
	globalShared "github.com/plsyro/data-pkg/shared"
)

var GrouperAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "GrouperAsResource",
	Version:   string(base.Alpha1),
	Plural:    "groupersasresources",
	Namespace: globalShared.BaseNamespace,
}

var AppWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "AppWorkloadAsResource",
	Version:   string(base.Alpha1),
	Plural:    "appsworkloadsasresources",
	Namespace: globalShared.BaseNamespace,
}

var BatchWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "BatchWorkloadAsResource",
	Version:   string(base.Alpha1),
	Plural:    "batchesworkloadsasresources",
	Namespace: globalShared.BaseNamespace,
}

var BridgeAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "BridgeAsResource",
	Version:   string(base.Alpha1),
	Plural:    "bridgesasresources",
	Namespace: globalShared.BaseNamespace,
}
