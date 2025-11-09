package auth

import (
	"github.com/plsyro/data/metadata/base"
	globalshared "github.com/plsyro/data/shared"
)

var AuthChallengeMetadata = base.Metadata{
	BaseGroup: string(base.Auth),
	Kind:      "AuthChallenge",
	Version:   string(base.Alpha1),
	Plural:    "authchallenges",
	Namespace: globalshared.BaseNamespace,
}

var UserCredentialMetadata = base.Metadata{
	BaseGroup: string(base.Auth),
	Kind:      "UserCredential",
	Version:   string(base.Alpha1),
	Plural:    "usercredentials",
	Namespace: globalshared.BaseNamespace,
}

var UserSessionMetadata = base.Metadata{
	BaseGroup: string(base.Auth),
	Kind:      "UserSession",
	Version:   string(base.Alpha1),
	Plural:    "usersessions",
	Namespace: globalshared.BaseNamespace,
}
