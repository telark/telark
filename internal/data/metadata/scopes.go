package metadata

import "github.com/plsyro/common-pkg/v2/data/global"

var ResourceAsGrouperMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsGrouper",
	Version:   string(alpha1),
	Plural:    "resourcesasgroupers",
	Namespace: global.BaseNamespace,
}

var ResourceAsWorkloadMetadata = Metadata{
	BaseGroup: string(Erpi),
	Kind:      "ResourceAsWorkload",
	Version:   string(alpha1),
	Plural:    "resourcesasworkloads",
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
