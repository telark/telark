package nats

import (
	"context"

	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	natsinit "github.com/telark/telark/internal/x-ware/nats/init"
	"github.com/telark/telark/services/discovery/internal/constants"
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
