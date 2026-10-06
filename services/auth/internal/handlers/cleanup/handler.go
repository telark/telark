package cleanup

import (
	"context"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"github.com/telark/telark/internal/rest/base"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	restconstants "github.com/telark/telark/internal/rest/constants"
	accessroleeps "github.com/telark/telark/internal/rest/endpoints/accessroles"
	groupeps "github.com/telark/telark/internal/rest/endpoints/groups"
	usereps "github.com/telark/telark/internal/rest/endpoints/users"
	"github.com/telark/telark/internal/rest/response"
	resputils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/auth/internal/authz"
	authclients "github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	coordcleanup "github.com/telark/telark/services/auth/internal/coordination/cleanup"
	sharedhelper "github.com/telark/telark/services/auth/internal/helpers/shared"
)

var (
	lg              = constants.GetLogger(constants.LoggerPrefixCleanup)
	ingressInstance *coordcleanup.Ingress
)

func InitIngress(i *coordcleanup.Ingress) {
	ingressInstance = i
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeUsers, authclients.GetUserClient().Client, usereps.DeleteUserByID, authz.GuardUserDelete)
}

func DeleteGroup(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeGroups, authclients.GetGroupClient().Client, groupeps.DeleteGroupByID, authz.GuardGroupDelete)
}

func DeleteAccessRole(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeRoles,
		authclients.GetAccessRoleClient().Client, accessroleeps.DeleteAccessRoleByID, authz.GuardRoleDelete)
}

func handleDelete(
	w http.ResponseWriter,
	r *http.Request,
	resourceType string,
	client *restshared.Client,
	endpoint base.Endpoint,
	guard func(ctx context.Context, id string) (int, error),
) {
	id, err := sharedhelper.GetPathParam(r, constants.IDPathParam)
	if err != nil || id == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), constants.IDPathParam)
		resputils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			msg, nil, err)
		return
	}

	if status, guardErr := guard(r.Context(), id); guardErr != nil {
		sharedhelper.SendRefusal(w, status, guardErr)
		return
	}

	// The exporter stamps the audit actor from this header; without it the stored editor stays.
	userID := r.Header.Get(constants.HeaderUserID)
	byID := client.WithParams(map[string]string{restconstants.IDParam: id})
	deleteResp := restshared.ExecuteRequestWithHeaders(byID, base.Delete, endpoint, nil, map[string]string{constants.HeaderUserID: userID})
	if deleteResp == nil {
		resputils.LogAndSendResponse(w, http.StatusBadGateway, response.OperationError,
			string(constants.MsgCleanupDeletingInProgress), nil, nil)
		return
	}
	if deleteResp.Status != http.StatusOK {
		resputils.LogAndSendResponse(w, deleteResp.Status, response.OperationError,
			sharedhelper.ExporterMessage(deleteResp.Message), nil, nil)
		return
	}

	if ingressInstance != nil {
		_, enqErr := ingressInstance.Enqueue(r.Context(), coordcleanup.EnqueueRequest{
			ResourceType: resourceType,
			ResourceID:   id,
			RequestedBy:  userID,
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
