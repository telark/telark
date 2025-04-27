package nats

import (
	"fmt"

	"github.com/plsyro/common-pkg/v2/global"
)

type Action string

const (
	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"
)

type Scope string

const (
	NAMESPACE  Scope = "namespaces"
	DEPLOYMENT Scope = "deployments"
)

func GetTopic(scope Scope, action Action) string {
	return fmt.Sprintf("%s.%s.%s", global.BaseNamespace, scope, action)
}
