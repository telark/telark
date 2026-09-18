package cleanup

import (
	"net/http"
	"slices"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func AddFinalizer(w http.ResponseWriter, r *http.Request) {
	mutateFinalizer(w, r, finalizerAdd)
}

func RemoveFinalizer(w http.ResponseWriter, r *http.Request) {
	mutateFinalizer(w, r, finalizerRemove)
}

type finalizerMutation int

const (
	finalizerAdd finalizerMutation = iota
	finalizerRemove
)

func mutateFinalizer(w http.ResponseWriter, r *http.Request, op finalizerMutation) {
	target, id, name, ok := extractFinalizerInputs(w, r)
	if !ok {
		return
	}

	lock := concurrency.GetLock(id)
	lock.Lock()
	defer lock.Unlock()

	getResult := api.GetCustomResourceByName(id, target.Metadata)
	if getResult.Status != http.StatusOK {
		responseutils.LogAndSendResponse(w, getResult.Status, response.OperationError,
			getResult.Message, nil, getResult.Error)
		return
	}

	current, ok := extractFinalizers(getResult.Data)
	if !ok {
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError,
			response.OperationError, string(globalerrors.ErrGetRes), nil, nil)
		return
	}

	next, changed := applyFinalizerMutation(current, name, op)
	if !changed {
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
			messageNoChange, map[string]any{constants.FieldFinalizers: current}, nil)
		return
	}

	patch := map[string]any{constants.MetadataField: map[string]any{constants.FieldFinalizers: next}}
	patchResult := api.PatchCustomResource(target.Metadata, id, patch)
	if patchResult.Status != http.StatusOK {
		responseutils.LogAndSendResponse(w, patchResult.Status, response.OperationError,
			patchResult.Message, nil, patchResult.Error)
		return
	}

	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		messageFinalizersUpdated, map[string]any{constants.FieldFinalizers: next}, nil)
}

func extractFinalizerInputs(
	w http.ResponseWriter,
	r *http.Request,
) (target resourceTarget, id, name string, ok bool) {
	target, ok = resolveTargetFromRequest(w, r)
	if !ok {
		return resourceTarget{}, constants.EmptyString, constants.EmptyString, false
	}
	id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
	if err != nil {
		return resourceTarget{}, constants.EmptyString, constants.EmptyString, false
	}
	body, err := requestutils.ParseRequestBody(r)
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusUnprocessableEntity, response.OperationError,
			string(globalerrors.ErrRestParseRequestBody), nil, err)
		return resourceTarget{}, constants.EmptyString, constants.EmptyString, false
	}
	rawName, isString := body[restconstants.FieldFinalizerName].(string)
	if !isString || rawName == constants.EmptyString {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			messageFinalizerNameRequired, nil, nil)
		return resourceTarget{}, constants.EmptyString, constants.EmptyString, false
	}
	return target, id, rawName, true
}

func applyFinalizerMutation(current []string, name string, op finalizerMutation) (next []string, changed bool) {
	present := slices.Contains(current, name)
	switch op {
	case finalizerAdd:
		if present {
			return current, false
		}
		return append(slices.Clone(current), name), true
	case finalizerRemove:
		if !present {
			return current, false
		}
		filtered := slices.DeleteFunc(slices.Clone(current), func(s string) bool { return s == name })
		return filtered, true
	}
	return current, false
}

func extractFinalizers(data any) ([]string, bool) {
	obj, ok := data.(*unstructured.Unstructured)
	if !ok || obj == nil {
		return nil, false
	}
	return obj.GetFinalizers(), true
}
