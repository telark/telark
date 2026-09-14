package cleanup

import (
	"net/http"

	globalerrors "github.com/telark/data/errors"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetCleanupViewByID(w http.ResponseWriter, r *http.Request) {
	target, ok := resolveTargetFromRequest(w, r)
	if !ok {
		return
	}
	id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
	if err != nil {
		return
	}
	result := api.GetCustomResourceByName(id, target.Metadata)
	if result.Status != http.StatusOK {
		responseutils.LogAndSendResponse(w, result.Status, response.OperationError,
			result.Message, nil, result.Error)
		return
	}
	obj, ok := result.Data.(*unstructured.Unstructured)
	if !ok || obj == nil {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError,
			response.OperationError, string(globalerrors.ErrGetRes), nil, nil)
		return
	}
	view := projectCleanupView(obj, target.RefKeys)
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		messageViewProjected, view, nil)
}

func ListCleanupViews(w http.ResponseWriter, r *http.Request) {
	target, ok := resolveTargetFromRequest(w, r)
	if !ok {
		return
	}
	result := api.ListCustomResources(target.Metadata)
	if result.Status != http.StatusOK {
		responseutils.LogAndSendResponse(w, result.Status, response.OperationError,
			result.Message, nil, result.Error)
		return
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok || list == nil {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError,
			response.OperationError, string(globalerrors.ErrGetRes), nil, nil)
		return
	}
	views := make([]resourcesshared.CleanupView, constants.DefaultInitValue, len(list.Items))
	for i := range list.Items {
		views = append(views, projectCleanupView(&list.Items[i], target.RefKeys))
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		messageViewsProjected, cleanupViewsResponse{Items: views}, nil)
}

type cleanupViewsResponse struct {
	Items []resourcesshared.CleanupView `json:"items"`
}
