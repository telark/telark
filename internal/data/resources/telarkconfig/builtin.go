package telarkconfig

import (
	"slices"

	"github.com/telark/telark/internal/data/constants"
)

var defaultExcludedNamespaces = []string{
	"default",
	"kube-system",
	"kube-public",
	"kube-node-lease",
}

func DefaultTelarkConfig() TelarkConfig {
	return TelarkConfig{
		ExcludedNamespaces: slices.Clone(defaultExcludedNamespaces),
		Snapshots:          SnapshotsConfig{MaxPerApp: constants.DefaultSnapshotsMaxPerApp},
		AI:                 AIConfig{Enabled: true, Model: constants.DefaultAnalyzerModel, AutoAnalyze: false},
		Cluster:            Cluster{},
		OIDC:               OIDCConfig{},
		SelfRegistration:   SelfRegistrationConfig{},
	}
}
