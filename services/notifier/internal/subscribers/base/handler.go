package base

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/internal/constants"
	natscore "github.com/telark/x-ware/nats/core"
)

func SharedExecuteHandler(
	m *nats.Msg,
	action natscore.Action,
	resourceKey string,
	getHandler func(natscore.Action) func(*nats.Msg) error,
	transformData func([]byte) ([]byte, error),
) error {
	handler := getHandler(action)
	logger.GetLogger(constants.PrefixManagerSubscriber).Debug(fmt.Sprintf(
		string(messages.SuccessNatsTopicMessageReceive), m.Subject))
	msgStr := natscore.GetParsedMessageHeader(m)
	if msgStr == constants.EmptyString {
		return fmt.Errorf("%s", errors.ErrNatsNoParsedMessageFoundInMetadata)
	}

	var msg natscore.Message
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToUnmarshalMsgData), err)
	}

	if action == natscore.Delete {
		return handler(m)
	}

	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToMarshalMsgData), err)
	}

	var transformed []byte
	if transformData != nil {
		transformed, err = transformData(dataBytes)
		if err != nil {
			return fmt.Errorf("%s: %w", errors.ErrNatsConvertMsgData, err)
		}
	} else {
		transformed = dataBytes
	}

	natscore.SetParsedMessageHeader(m, resourceKey, string(transformed))
	return handler(m)
}

func (*BaseSubscriber) HandleUnknown(m *nats.Msg) error {
	_ = AckWithLog(m, m.Subject, constants.EmptyString, false)
	return nil
}
