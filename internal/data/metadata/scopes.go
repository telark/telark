package metadata

import "github.com/plsyro/common-pkg/v2/data/global"

var ResourceAsGrouperMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsGrouper",
	Version:   string(alpha1),
	Plural:    "resourcesasgroupers",
	Namespace: global.BaseNamespace,
}

var ResourceAsAppWorkloadMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsAppWorkload",
	Version:   string(alpha1),
	Plural:    "resourcesasappsworkloads",
	Namespace: global.BaseNamespace,
}

var ResourceAsBatchWorkloadMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsBatchWorkload",
	Version:   string(alpha1),
	Plural:    "resourcesasbatchesworkloads",
	Namespace: global.BaseNamespace,
}

var ResourceAsBridgeMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsBridge",
	Version:   string(alpha1),
	Plural:    "resourcesasbridges",
	Namespace: global.BaseNamespace,
}

var ResourceAsInsightMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsInsight",
	Version:   string(alpha1),
	Plural:    "resourcesasinsights",
	Namespace: global.BaseNamespace,
}
