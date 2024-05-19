package metadata

import "github.com/plsyro/common-pkg/v2/data/global"

var PocketPlanMetadata = Metadata{
	BaseGroup: string(Plan),
	Kind:      "PocketPlan",
	Version:   string(alpha2),
	Plural:    "pocketsplans",
	Namespace: global.BaseNamespace,
}

var CoinPlanMetadata = Metadata{
	BaseGroup: string(Plan),
	Kind:      "CoinPlan",
	Version:   string(alpha2),
	Plural:    "coinsplans",
	Namespace: global.BaseNamespace,
}

var SyncPlanMetadata = Metadata{
	BaseGroup: string(Plan),
	Kind:      "SyncPlan",
	Version:   string(alpha1),
	Plural:    "syncsplans",
	Namespace: global.BaseNamespace,
}

var EnvironmentInsightMetadata = Metadata{
	BaseGroup: string(Insights),
	Kind:      "EnvironmentInsight",
	Version:   string(alpha2),
	Plural:    "environmentsinsights",
	Namespace: global.BaseNamespace,
}
