package group

import (
	"net/http"

	metadata "github.com/telark/data/metadata/v1alpha1"
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func ValidateAndPrepareGroup(group *groupdata.Group, w http.ResponseWriter) error {
	if err := sharedutils.ValidateRequiredField(group.Name, string(constants.ErrGroupNameCannotBeEmpty)); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			err,
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
