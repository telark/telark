package globalconfig

import (
	"slices"

	"github.com/telark/data/constants"
)

const defaultUserSetting = "default"

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
			Theme:                defaultUserSetting,
			Density:              defaultUserSetting,
			UIViewSize:           defaultUserSetting,
			FetchIntervalSeconds: constants.DefaultFetchIntervalSeconds,
		},
		Snapshots: SnapshotsConfig{MaxPerApp: constants.DefaultSnapshotsMaxPerApp},
		AI:        AIConfig{},
		Cluster:   Cluster{},
		OIDC:      OIDCConfig{},
	}
}
