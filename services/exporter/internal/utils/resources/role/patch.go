package role

import (
	"net/http"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func GetExistingRoleForPatch(w http.ResponseWriter, roleID string) (*roledata.AccessRole, bool) {
	existingResource, ok := FindRoleByIDOrRespond(w, roleID)
	if !ok {
		return nil, false
	}

	sharedutils.ProjectDeletionTimestamp(existingResource)
	existingRole, err := ExtractRoleFromUnstructured(existingResource)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(constants.ErrRoleExtractData),
			nil,
			err,
		)
		return nil, false
	}

	return existingRole, true
}

func ExtractAndMergeRoleForPatch(existingRole *roledata.AccessRole, body map[string]any, w http.ResponseWriter) (*roledata.AccessRole, bool) {
	delete(body, constants.FieldPriority)
	delete(body, constants.FieldVersion)
	newRole, err := ExtractRoleSpecFromRequestBody(body)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return nil, false
	}

	mergedRole := MergeRoleAndPreparePatchBody(existingRole, newRole, body)
	delete(body, constants.FieldCreationDate)
	if mergedRole.Type != roledata.RoleTypeBuiltIn {
		if err := ValidatePriorityCapOrRespond(w, mergedRole.Priority); err != nil {
			return nil, false
		}
	}
	return mergedRole, true
}
