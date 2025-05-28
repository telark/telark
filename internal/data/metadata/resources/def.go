package resources

import (
	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/metadata/base"
)

var GrouperAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "GrouperAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "groupersasresources",
	Namespace: common.BaseNamespace,
}

var AppWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "AppWorkloadAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "appsworkloadsasresources",
	Namespace: common.BaseNamespace,
}

var BatchWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "BatchWorkloadAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "batchesworkloadsasresources",
	Namespace: common.BaseNamespace,
}

var BridgeAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "BridgeAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "bridgesasresources",
	Namespace: common.BaseNamespace,
}

var InsightAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.ERPI),
	Kind:      "InsightAsResource",
	Version:   string(base.ALPHA_1),
	Plural:    "insightsasresources",
	Namespace: common.BaseNamespace,
}
