package globalconfig

import (
	"slices"

	"github.com/telark/data/constants"
)

var defaultExcludedNamespaces = []string{
	"default",
	"kube-system",
	"kube-public",
	"kube-node-lease",
}

func DefaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		ExcludedNamespaces: slices.Clone(defaultExcludedNamespaces),
		UserSettings: UserSettings{
			FetchIntervalSeconds: constants.DefaultFetchIntervalSeconds,
		},
		Snapshots: SnapshotsConfig{MaxPerApp: constants.DefaultSnapshotsMaxPerApp},
		AI:        AIConfig{Enabled: true, Model: constants.DefaultAnalyzerModel, AutoAnalyze: false},
		Cluster:   Cluster{},
		OIDC:      OIDCConfig{},
	}
}
