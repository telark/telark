package resources

import (
	"github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var ClusterInsightAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "ClusterInsightAsResource",
	Version:   string(base.Alpha1),
	Plural:    "clusterinsightsasresources",
	Namespace: globalshared.BaseNamespace,
}

var UserAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "UserAsResource",
	Version:   string(base.Alpha1),
	Plural:    "usersasresources",
	Namespace: globalshared.BaseNamespace,
}

var GroupAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "GroupAsResource",
	Version:   string(base.Alpha1),
	Plural:    "groupsasresources",
	Namespace: globalshared.BaseNamespace,
}

var RoleAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "RoleAsResource",
	Version:   string(base.Alpha1),
	Plural:    "rolesasresources",
	Namespace: globalshared.BaseNamespace,
}

var ApplicationAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "ApplicationAsResource",
	Version:   string(base.Alpha1),
	Plural:    "applicationsasresources",
	Namespace: globalshared.BaseNamespace,
}

var GlobalConfigMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "GlobalConfig",
	Version:   string(base.Alpha1),
	Plural:    "globalconfigs",
	Namespace: globalshared.BaseNamespace,
}
