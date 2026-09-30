package applications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	appresource "github.com/telark/telark/internal/data/resources/application"
	resourceshared "github.com/telark/telark/internal/data/resources/shared"
	restconstants "github.com/telark/telark/internal/rest/constants"
	"github.com/telark/telark/internal/rest/response"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/notifier/internal/constants"
	"github.com/telark/telark/services/notifier/internal/subscribers/base"
)

func (s *ApplicationSubscriber) handleUpdate(m *nats.Msg) error {
	msg, dataMap, err := parseNatsMessageToMap(m)
	if err != nil {
		return base.AckWithLog(m, m.Subject, err.Error(), true)
	}
	if msg.Scope == constants.EmptyString {
		return base.AckWithLog(m, m.Subject, string(errors.ErrNatsNoScopeFoundInUpdateMessage), true)
	}

	patchBody, resourceName, err := base.BuildPatchBodyFromScope(
		msg.Scope,
		dataMap,
		strings.ToLower(string(resourceshared.Application)),
		msg.ResourceName,
	)
	if err != nil {
		return base.AckWithLog(m, m.Subject, err.Error(), true)
	}

	resp := &base.GenericResponseAdapter{Resp: s.client.PatchApplicationByName(resourceName, patchBody)}
	if resp.GetStatus() == http.StatusNotFound {
		app, err := mapToApplication(dataMap, resourceName)
		if err != nil {
			return base.AckWithLog(m, m.Subject, err.Error(), true)
		}
		resp = &base.GenericResponseAdapter{Resp: s.client.CreateApplication(&app)}
	}
	if resp.GetStatus() == http.StatusOK {
		_ = base.AckWithLog(m, m.Subject, fmt.Sprintf(string(messages.SuccessNatsPatchApplication), resourceName), false)
		return nil
	}

	logMsg := fmt.Sprintf(string(errors.ErrNatsFailedToPatchApplication), resourceName, resp.GetMessage())
	if base.TransientStatus(resp.GetStatus()) {
		return s.NakWithLog(m, m.Subject, logMsg)
	}
	return base.AckWithLog(m, m.Subject, logMsg, true)
}

// Discovery's reset owns the cleanup: its Redis state plus the exporter delete of the CR and snapshots.
func (s *ApplicationSubscriber) handleDelete(m *nats.Msg) error {
	return s.ExecuteDeleteHandler(
		m,
		s.resetApplication,
		string(messages.SuccessNatsDeleteApplication),
		string(errors.ErrNatsFailedToDeleteApplication),
	)
}

func (s *ApplicationSubscriber) resetApplication(name string) base.GenericResponse {
	resp, err := s.client.ResetApplicationByName(name)
	if err != nil {
		resp = &response.GenericResponse{Status: statusFromError(err), Message: err.Error()}
	}
	return &base.GenericResponseAdapter{Resp: resp}
}

// The client folds a non-200 answer into ErrUnexpectedStatus; any other error
// had no HTTP answer at all and maps to 0, which is transient.
func statusFromError(err error) int {
	status := constants.DefaultInitValue
	var detail string
	_, _ = fmt.Sscanf(err.Error(), string(restconstants.ErrUnexpectedStatus), &status, &detail)
	return status
}

func parseNatsMessageToMap(m *nats.Msg) (*natscore.Message, map[string]any, error) {
	msgStr := natscore.GetParsedMessageHeader(m)
	if msgStr == constants.EmptyString {
		return nil, nil, fmt.Errorf("%s", errors.ErrNatsNoParsedMessageFoundInMetadata)
	}
	var msg natscore.Message
	if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
		return nil, nil, fmt.Errorf(string(errors.ErrNatsHandleMsg), m.Subject, err)
	}
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		return nil, nil, fmt.Errorf(string(errors.ErrNatsHandleMsg), m.Subject, err)
	}
	var dataMap map[string]any
	if err := json.Unmarshal(dataBytes, &dataMap); err != nil {
		return nil, nil, fmt.Errorf(string(errors.ErrNatsHandleMsg), m.Subject, err)
	}
	return &msg, dataMap, nil
}

func mapToApplication(dataMap map[string]any, fallbackName string) (appresource.Application, error) {
	b, err := json.Marshal(dataMap)
	if err != nil {
		return appresource.Application{}, err
	}
	var app appresource.Application
	if err := json.Unmarshal(b, &app); err != nil {
		return appresource.Application{}, err
	}
	if app.Name == constants.EmptyString {
		app.Name = fallbackName
	}
	return app, nil
}
