package base

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/notifier/internal/constants"
	natscore "github.com/telark/x-ware/nats/core"
)

func (s *BaseSubscriber) ExecuteDeleteHandler(
	m *nats.Msg,
	deleteFunc DeleteFunc,
	successMsgTmpl string,
	errorDeleteTmpl string,
) error {
	msgStr := natscore.GetParsedMessageHeader(m)
	if msgStr == constants.EmptyString {
		return AckWithLog(m, m.Subject, string(errors.ErrNatsNoParsedMessageFoundInMetadata), true)
	}

	var msg natscore.Message
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return AckWithLog(m, m.Subject, fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err), true)
	}

	resourceName := msg.ResourceName
	if resourceName == constants.EmptyString {
		return AckWithLog(m, m.Subject, string(errors.ErrNatsNoResourceNameFoundInDeleteMessage), true)
	}

	response := deleteFunc(resourceName)

	// A 404 means the resource is already gone, which is what the delete wanted.
	if status := response.GetStatus(); status != http.StatusOK && status != http.StatusNotFound {
		logMsg := fmt.Sprintf(errorDeleteTmpl, resourceName, response.GetMessage())
		if TransientStatus(status) {
			return s.NakWithLog(m, m.Subject, logMsg)
		}
		return AckWithLog(m, m.Subject, logMsg, true)
	}

	_ = AckWithLog(m, m.Subject, fmt.Sprintf(successMsgTmpl, resourceName), false)
	return nil
}
