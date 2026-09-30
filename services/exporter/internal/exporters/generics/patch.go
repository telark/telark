package generics

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GenericPatchCustomResource(w http.ResponseWriter, md metadata.Metadata, name string, patchData map[string]any) {
	if len(patchData) == constants.DefaultInitValue {
		sendInvalidPatchBodyResponse(w)
		return
	}

	exists, err := api.CheckCustomResourceExistsByName(name, md)
	if err != nil {
		status := sharedutils.StatusForK8sError(err)
		sharedutils.LogByStatusAndSend(w, status, response.OperationError, string(globalerrors.ErrCheckResExistence), nil, err)
		return
	}

	if exists {
		result := sharedutils.PatchCustomResource(md, name, patchData)
		if result.Status != http.StatusOK {
			errorMsg := sharedutils.GenerateResourceError(globalerrors.ErrUpdateRes, name, result.Error)
			sharedutils.LogByStatusAndSend(w, result.Status, response.OperationError, errorMsg, nil, result.Error)
			return
		}

		resource, ok := result.Data.(*unstructured.Unstructured)
		if !ok {
			responseutils.LogAndSendResponse(
				w,
				http.StatusInternalServerError,
				response.OperationError,
				string(globalerrors.ErrGetRes),
				nil,
				errors.New(string(constants.ErrInvalidResourceTypeReturned)),
			)
			return
		}

		filterAndRespond(w, resource, messages.SuccessUpdateRes)
	} else {
		errorMsg := fmt.Sprintf(string(globalerrors.ErrUpdateRes), name, "resource not found")
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			errorMsg,
			nil,
			nil,
		)
	}
}

func sendInvalidPatchBodyResponse(w http.ResponseWriter) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusBadRequest,
		response.OperationUnprocessed,
		string(globalerrors.ErrRestEmptyRequestBody),
		nil,
		nil,
	)
}
