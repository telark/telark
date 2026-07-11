package publisher

import (
	resourceshared "github.com/telark/data/resources/shared"
	natscore "github.com/telark/x-ware/nats/core"
)

type PublishUpdateParams struct {
	Name    string
	Scope   resourceshared.DataScope
	Group   natscore.Group
	Data    any
	ResType resourceshared.Type
}
