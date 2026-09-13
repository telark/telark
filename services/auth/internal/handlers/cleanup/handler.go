package cleanup

import (
	"fmt"
	"net/http"

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
	handleDelete(w, r, finalizers.ResourceTypeUsers, deleteUserBusiness)
}

func DeleteGroup(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeGroups, deleteGroupBusiness)
}

func DeleteRole(w http.ResponseWriter, r *http.Request) {
	handleDelete(w, r, finalizers.ResourceTypeRoles, deleteRoleBusiness)
}

type businessDeleteFn func(id string) *response.GenericResponse

func handleDelete(w http.ResponseWriter, r *http.Request, resourceType string, deleteFn businessDeleteFn) {
	id, err := sharedhelper.GetPathParam(r, constants.IDPathParam)
	if err != nil || id == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), constants.IDPathParam)
		resputils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			msg, nil, err)
		return
	}

	deleteResp := deleteFn(id)
	if deleteResp == nil {
		resputils.LogAndSendResponse(w, http.StatusBadGateway, response.OperationError,
			string(constants.MsgCleanupDeletingInProgress), nil, nil)
		return
	}
	if deleteResp.Status != http.StatusOK && deleteResp.Status != http.StatusNotFound {
		resputils.LogAndSendResponse(w, deleteResp.Status, response.OperationError,
			deleteResp.Message, nil, nil)
		return
	}

	if ingressInstance != nil {
		_, enqErr := ingressInstance.Enqueue(r.Context(), coordcleanup.EnqueueRequest{
			ResourceType: resourceType,
			ResourceID:   id,
			RequestedBy:  requesterID(r),
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

func requesterID(r *http.Request) string {
	return r.Header.Get(constants.HeaderUserID)
}

func deleteUserBusiness(id string) *response.GenericResponse {
	return authclients.GetUserClient().DeleteUserByID(id)
}

func deleteGroupBusiness(id string) *response.GenericResponse {
	return authclients.GetGroupClient().DeleteGroupByID(id)
}

func deleteRoleBusiness(id string) *response.GenericResponse {
	return authclients.GetRoleClient().DeleteRoleByID(id)
}
