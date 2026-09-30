package publisher

import (
	resourceshared "github.com/telark/telark/internal/data/resources/shared"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
)

type PublishUpdateParams struct {
	Name    string
	Scope   resourceshared.DataScope
	Group   natscore.Group
	Data    any
	ResType resourceshared.Type
}
