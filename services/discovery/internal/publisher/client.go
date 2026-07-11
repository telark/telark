package publisher

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/constants"
	natscore "github.com/telark/x-ware/nats/core"
	natstreams "github.com/telark/x-ware/nats/streams"
)

var (
	mu sync.RWMutex
	lg = constants.GetLogger(constants.LoggerPrefixEventPublisher)
)

func PublishEvent(natsClient *natscore.NATSClient, topic string, msg any) (*nats.PubAck, error) {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedMarshalPayload), err))
		return nil, err
	}

	var ack *nats.PubAck
	var publishErr error

	circuitErr := circuitbreaker.ExecuteNATS(func() error {
		ack, publishErr = attemptPublish(natsClient, topic, data)
		return publishErr
	})

	if circuitErr != nil {
		return nil, circuitErr
	}

	return ack, publishErr
}

func attemptPublish(natsClient *natscore.NATSClient, topic string, data []byte) (*nats.PubAck, error) {
	maxRetries := constants.NatsPublishMaxRetries
	for i := range maxRetries {
		if !isNatsClientHealthy(natsClient) {
			err := fmt.Errorf(string(constants.ErrNatsClientNotConnected), string(errors.ErrNatsConnectionFailed))
			lg.Error(err.Error())
			if i < maxRetries-constants.RetryBackoff {
				time.Sleep(calculateNatsRetryDelay(i))
			}
			continue
		}

		ack, err := publishWithTimeout(natsClient, topic, data)
		if err == nil && ack != nil {
			return ack, nil
		}

		lg.Error(fmt.Sprintf(string(constants.ErrNatsPublishRetry), i+1, maxRetries, err))
		if i < maxRetries-constants.RetryBackoff {
			time.Sleep(calculateNatsRetryDelay(i))
		}
	}

	return nil, fmt.Errorf(string(constants.ErrNatsTopicPublishAfterAttempts),
		string(errors.ErrNatsTopicPublish), topic, maxRetries)
}

func isNatsClientHealthy(natsClient *natscore.NATSClient) bool {
	if natsClient == nil {
		return false
	}
	if natsClient.Conn == nil {
		return false
	}
	return natsClient.Conn.IsConnected()
}

func publishWithTimeout(natsClient *natscore.NATSClient, topic string, data []byte) (*nats.PubAck, error) {
	return natstreams.PublishMessage(natsClient, topic, data)
}

func calculateNatsRetryDelay(attempt int) time.Duration {
	base := constants.NatsPublishRetryDelay
	if attempt >= constants.TwoValue {
		base *= constants.HighLoadBackoff
	}
	exp := min(time.Duration(1<<attempt)*base, constants.NatsPublishMaxRetryDelay)
	var jitter time.Duration
	if n, err := rand.Int(rand.Reader, big.NewInt(int64(base))); err == nil {
		jitter = time.Duration(n.Int64())
	}
	return exp + jitter
}
