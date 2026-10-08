package group

import (
	"net/http"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func ValidateAndPrepareGroup(group *groupdata.Group, w http.ResponseWriter) error {
	if err := sharedutils.ValidateRequiredField(group.Name, string(constants.ErrGroupNameCannotBeEmpty)); err != nil {
		responseutils.SendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
		)
		return err
	}

	groupID, err := resourcesshared.GenerateUniqueResourceID(
		metadata.GroupMetadata,
		constants.GroupIDConfig,
	)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return err
	}
	group.ID = groupID
	return nil
}
