package resources

import (
	"github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var GrouperAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "GrouperAsResource",
	Version:   string(base.Alpha1),
	Plural:    "groupersasresources",
	Namespace: globalshared.BaseNamespace,
}

var AppWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "AppWorkloadAsResource",
	Version:   string(base.Alpha1),
	Plural:    "appsworkloadsasresources",
	Namespace: globalshared.BaseNamespace,
}

var BatchWorkloadAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "BatchWorkloadAsResource",
	Version:   string(base.Alpha1),
	Plural:    "batchesworkloadsasresources",
	Namespace: globalshared.BaseNamespace,
}

var BridgeAsResourceMetadata = base.Metadata{
	BaseGroup: string(base.Erpi),
	Kind:      "BridgeAsResource",
	Version:   string(base.Alpha1),
	Plural:    "bridgesasresources",
	Namespace: globalshared.BaseNamespace,
}

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
