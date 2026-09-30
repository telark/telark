package cleanup

import (
	"net/http"

	globalerrors "github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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
		responseutils.LogAndSendResponse(w, sharedutils.StatusForResult(result), response.OperationError,
			result.Message, nil, result.Error)
		return
	}
	obj, ok := result.Data.(*unstructured.Unstructured)
	if !ok || obj == nil {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError,
			response.OperationError, string(globalerrors.ErrGetRes), nil, nil)
		return
	}
	hidden, ok := hiddenUsers(w, r)
	if !ok {
		return
	}
	if hidden[obj.GetName()] && target.Metadata.Kind == metadata.UserMetadata.Kind {
		responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrUserNotFound), nil, nil)
		return
	}
	view := projectCleanupView(obj, target.RefKeys, hidden)
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		messageViewProjected, view, nil)
}

// The views name users by id and list group members; a restricted caller
// sees administrators in neither. nil means nothing to hide.
func hiddenUsers(w http.ResponseWriter, r *http.Request) (map[string]bool, bool) {
	if !authz.Restricted(r) {
		return nil, true
	}
	hidden, err := authz.HiddenUserIDs()
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusServiceUnavailable, response.OperationUnavailable,
			string(constants.ErrResourceLookupFailed), nil, err)
		return nil, false
	}
	return hidden, true
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
	hidden, ok := hiddenUsers(w, r)
	if !ok {
		return
	}
	views := make([]resourcesshared.CleanupView, constants.DefaultInitValue, len(list.Items))
	for i := range list.Items {
		if hidden[list.Items[i].GetName()] && target.Metadata.Kind == metadata.UserMetadata.Kind {
			continue
		}
		views = append(views, projectCleanupView(&list.Items[i], target.RefKeys, hidden))
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		messageViewsProjected, cleanupViewsResponse{Items: views}, nil)
}

type cleanupViewsResponse struct {
	Items []resourcesshared.CleanupView `json:"items"`
}
