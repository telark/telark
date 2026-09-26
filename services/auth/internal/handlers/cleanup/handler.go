package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/telark/auth/internal/authz"
	authclients "github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	coordcleanup "github.com/telark/auth/internal/coordination/cleanup"
	sharedhelper "github.com/telark/auth/internal/helpers/shared"
	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/resources/finalizers"
	"github.com/telark/rest/response"
	resputils "github.com/telark/rest/utils/response"
)

var (
	lg              = constants.GetLogger(constants.LoggerPrefixCleanup)
	ingressInstance *coordcleanup.Ingress
)

func InitIngress(i *coordcleanup.Ingress) {
	ingressInstance = i
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeUsers, authclients.GetUserClient().DeleteUserByID, authz.GuardUserDelete)
}

func DeleteGroup(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeGroups, authclients.GetGroupClient().DeleteGroupByID, nil)
}

func DeleteAccessRole(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeRoles, authclients.GetAccessRoleClient().DeleteAccessRoleByID, nil)
}

func handleDelete(
	w http.ResponseWriter,
	r *http.Request,
	resourceType string,
	deleteFn func(id string) *response.GenericResponse,
	guard func(ctx context.Context, id string) (int, error),
) {
	id, err := sharedhelper.GetPathParam(r, constants.IDPathParam)
	if err != nil || id == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), constants.IDPathParam)
		resputils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			msg, nil, err)
		return
	}

	if guard != nil {
		if status, guardErr := guard(r.Context(), id); guardErr != nil {
			resputils.LogAndSendResponse(w, status, response.OperationError, guardErr.Error(), nil, nil)
			return
		}
	}

	deleteResp := deleteFn(id)
	if deleteResp == nil {
		resputils.LogAndSendResponse(w, http.StatusBadGateway, response.OperationError,
			string(constants.MsgCleanupDeletingInProgress), nil, nil)
		return
	}
	if deleteResp.Status != http.StatusOK {
		resputils.LogAndSendResponse(w, deleteResp.Status, response.OperationError,
			exporterMessage(deleteResp.Message), nil, nil)
		return
	}

	if ingressInstance != nil {
		_, enqErr := ingressInstance.Enqueue(r.Context(), coordcleanup.EnqueueRequest{
			ResourceType: resourceType,
			ResourceID:   id,
			RequestedBy:  r.Header.Get(constants.HeaderUserID),
		})
		if enqErr != nil {
			lg.Warn(enqErr.Error())
		}
	}

	resputils.LogAndSendResponse(w, http.StatusAccepted, response.OperationSuccess,
		string(constants.SuccessCleanupAccepted), map[string]any{
			constants.IDPathParam:              id,
			constants.CleanupFieldResourceType: resourceType,
		}, nil)
}

// The rest client wraps a refused delete as "HTTP <status>: <json body>"; the
// caller wants the exporter's own message.
func exporterMessage(wrapped string) string {
	start := strings.Index(wrapped, constants.JSONObjectStart)
	if start < constants.DefaultInitValue {
		return wrapped
	}
	var inner response.GenericResponse
	if err := json.Unmarshal([]byte(wrapped[start:]), &inner); err != nil || inner.Message == constants.EmptyString {
		return wrapped
	}
	return inner.Message
}
