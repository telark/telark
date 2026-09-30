package shared

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type (
	FindResourceFunc func() (*unstructured.Unstructured, error)
	NotFoundError    error
)

func FindResourceByIDOrRespond(
	w http.ResponseWriter,
	resourceID string,
	md metadata.Metadata,
	notFoundErr dataerrors.Error,
) (*unstructured.Unstructured, bool) {
	return findResourceOrRespond(w, func() (*unstructured.Unstructured, error) {
		return FindResourceByID(resourceID, md, notFoundErr)
	}, notFoundErr)
}

func FindResourceByID(
	resourceID string,
	md metadata.Metadata,
	notFoundErr dataerrors.Error,
) (*unstructured.Unstructured, error) {
	resourceResult := api.GetCustomResourceByName(resourceID, md)
	if err := sharedutils.ErrorForResult(resourceResult, notFoundErr); err != nil {
		return nil, err
	}

	resource, ok := resourceResult.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New(string(notFoundErr))
	}

	return resource, nil
}

func findResourceOrRespond(
	w http.ResponseWriter,
	findFunc FindResourceFunc,
	notFoundErr dataerrors.Error,
) (*unstructured.Unstructured, bool) {
	resource, err := findFunc()
	if err != nil {
		if err.Error() == string(notFoundErr) {
			responseutils.LogAndSendResponse(
				w,
				http.StatusNotFound,
				response.OperationNotFound,
				string(notFoundErr),
				nil,
				nil,
			)
			return nil, false
		}

		responseutils.LogAndSendResponse(
			w,
			sharedutils.StatusForError(err, http.StatusInternalServerError),
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return nil, false
	}
	return resource, true
}
