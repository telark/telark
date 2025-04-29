package resources

import (
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/metadata/base"
)

var GrouperAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "GrouperAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "groupersasresources",
	Namespace: global.BaseNamespace,
}

var AppWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "AppWorkloadAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "appsworkloadsasresources",
	Namespace: global.BaseNamespace,
}

var BatchWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "BatchWorkloadAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "batchesworkloadsasresources",
	Namespace: global.BaseNamespace,
}

var BridgeAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "BridgeAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "bridgesasresources",
	Namespace: global.BaseNamespace,
}

var InsightAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "InsightAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "insightsasresources",
	Namespace: global.BaseNamespace,
}
