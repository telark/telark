package nats

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/discovery/internal/constants"
)

var (
	natsManager natscore.NatsManagerInterface
	current     atomic.Pointer[natscore.NATSClient]
	firstDial   sync.Once
	redialing   atomic.Bool
	dialFailing atomic.Bool
)

func init() {
	natsManager = natscore.NewNatsManager()
}

// Informer flushes and coordination jobs call this on every run: only the first call (startup
// prewarm) waits for a dial, and a lost connection is redialed in the background meanwhile.
func GetClient() *natscore.NATSClient {
	if os.Getenv(natscore.EnvNatsHost) == constants.EmptyString {
		return nil
	}
	firstDial.Do(dial)
	if nc := current.Load(); nc != nil && nc.Conn.IsConnected() {
		return nc
	}
	if redialing.CompareAndSwap(false, true) {
		go func() {
			defer redialing.Store(false)
			dial()
		}()
	}
	return nil
}

// One line per outage, not one per redial.
func dial() {
	nc, err := natsManager.GetClient()
	if err != nil {
		if !dialFailing.Swap(true) {
			constants.GetLogger(constants.LoggerPrefixEventPublisher).Warn(fmt.Sprintf(string(constants.WarnNatsDialFailed), err))
		}
		return
	}
	dialFailing.Store(false)
	current.Store(nc)
}
