package metadata

import "github.com/plsyro/common-pkg/v2/data/global"

// Resources
var GrouperAsResourceMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "GrouperAsResource",
	Version:   string(alpha1),
	Plural:    "groupersasresources",
	Namespace: global.BaseNamespace,
}

var AppWorkloadAsResourceMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "AppWorkloadAsResource",
	Version:   string(alpha1),
	Plural:    "appworkloadsasresources",
	Namespace: global.BaseNamespace,
}

var BatchWorkloadAsResourceMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "BatchWorkloadAsResource",
	Version:   string(alpha1),
	Plural:    "batchworkloadsasresources",
	Namespace: global.BaseNamespace,
}

var BridgeAsResourceMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "BridgeAsResource",
	Version:   string(alpha1),
	Plural:    "bridgesasresources",
	Namespace: global.BaseNamespace,
}

var InsightAsResourceMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "InsightAsResource",
	Version:   string(alpha1),
	Plural:    "resourcesasinsights",
	Namespace: global.BaseNamespace,
}

// Capabilities
var MaintenanceAsCapabilityMetadata = Metadata{
	BaseGroup: string(Caps),
	Kind:      "MaintenanceAsCapability",
	Version:   string(alpha1),
	Plural:    "maintenancesascapabilities",
	Namespace: global.BaseNamespace,
}
