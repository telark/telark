package role

import (
	"net/http"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GetExistingRoleForPatch(w http.ResponseWriter, roleID string) (*roledata.RoleAsResource, bool) {
	existingResource, ok := FindRoleByIDOrRespond(w, roleID)
	if !ok {
		return nil, false
	}

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

func ValidatePatchRequest(existingRole *roledata.RoleAsResource, body map[string]any, w http.ResponseWriter) bool {
	return ValidateProtectionFlags(existingRole, body, w)
}

func ExtractAndMergeRoleForPatch(existingRole *roledata.RoleAsResource, body map[string]any, w http.ResponseWriter) bool {
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
		return false
	}

	mergedRole := MergeRoleAndPreparePatchBody(existingRole, newRole, body)
	delete(body, constants.FieldCreationDate)
	if mergedRole.Type != roledata.RoleTypeBuiltIn {
		if err := ValidatePriorityCapOrRespond(w, mergedRole.Priority); err != nil {
			return false
		}
	}
	return true
}
