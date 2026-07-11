package nats

import (
	"context"

	"github.com/telark/discovery/constants"
	natscore "github.com/telark/x-ware/nats/core"
	natsinit "github.com/telark/x-ware/nats/init"
)

var natsManager natscore.NatsManagerInterface

func init() {
	natsManager = natscore.NewNatsManager()
}

func GetClient() (*natscore.NATSClient, error) {
	if natsManager == nil {
		return nil, nil
	}
	lg := constants.GetLogger(constants.LoggerPrefixEventPublisher)
	ctx, cancel := context.WithTimeout(context.Background(), constants.NATSConnectTimeout)
	defer cancel()
	nc := natsinit.NewClientWithRetry(
		ctx,
		func() (*natscore.NATSClient, error) { return natsManager.GetClient() },
		natsinit.RetryConfig{},
		lg,
	)
	return nc, nil
}
