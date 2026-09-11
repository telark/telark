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

type (
	PatchFunc       func(resourceName string, patchBody map[string]any) GenericResponse
	DeleteFunc      func(resourceName string) GenericResponse
	GenericResponse interface {
		GetStatus() int
		GetMessage() string
	}
)

func ExecuteUpdateHandler(
	m *nats.Msg,
	resourceNameKey string,
	buildPatchBody func(scope string, dataMap map[string]any, resourceNameKey string,
		resourceNameFromMsg string) (map[string]any, string, error),
	patchFunc PatchFunc,
	successMsgTmpl string,
	errorPatchTmpl string,
) error {
	msgStr := natscore.GetParsedMessageHeader(m)
	if msgStr == "" {
		return AckWithLog(m, m.Subject, string(errors.ErrNatsNoParsedMessageFoundInMetadata), true)
	}

	var msg natscore.Message
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return AckWithLog(m, m.Subject, fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err), true)
	}

	scope := msg.Scope
	if scope == constants.EmptyString {
		return AckWithLog(m, m.Subject, string(errors.ErrNatsNoScopeFoundInUpdateMessage), true)
	}

	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		return AckWithLog(m, m.Subject, fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err), true)
	}

	var dataMap map[string]any
	if err := json.Unmarshal(dataBytes, &dataMap); err != nil {
		return AckWithLog(m, m.Subject, fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err), true)
	}

	patchBody, resourceName, err := buildPatchBody(scope, dataMap, resourceNameKey, msg.ResourceName)
	if err != nil {
		return AckWithLog(m, m.Subject, err.Error(), true)
	}

	response := patchFunc(resourceName, patchBody)

	if response.GetStatus() != http.StatusOK {
		return AckWithLog(m, m.Subject, fmt.Sprintf(errorPatchTmpl, resourceName, response.GetMessage()), true)
	}

	_ = AckWithLog(m, m.Subject, fmt.Sprintf(successMsgTmpl, resourceName), false)
	return nil
}

func ExecuteDeleteHandler(
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

	if response.GetStatus() != http.StatusOK {
		return AckWithLog(m, m.Subject, fmt.Sprintf(errorDeleteTmpl, resourceName, response.GetMessage()), true)
	}

	_ = AckWithLog(m, m.Subject, fmt.Sprintf(successMsgTmpl, resourceName), false)
	return nil
}
