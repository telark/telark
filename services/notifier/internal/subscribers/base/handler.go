package base

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/logger"
	"github.com/telark/telark/internal/data/messages"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/notifier/internal/constants"
)

func SharedExecuteHandler(
	m *nats.Msg,
	action natscore.Action,
	getHandler func(natscore.Action) func(*nats.Msg) error,
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

	return handler(m)
}

func (*BaseSubscriber) HandleUnknown(m *nats.Msg) error {
	_ = AckWithLog(m, m.Subject, constants.EmptyString, false)
	return nil
}
