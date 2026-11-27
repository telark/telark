package cleanup

import (
	"net/http"
	"slices"

	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/kcore/crds/api"
	restconstants "github.com/telark/telark/internal/rest/constants"
	cleanupeps "github.com/telark/telark/internal/rest/endpoints/cleanup"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func AddFinalizer(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		mutateFinalizer(w, r, optimizer, finalizerAdd)
	}
}

func RemoveFinalizer(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		mutateFinalizer(w, r, optimizer, finalizerRemove)
	}
}

type finalizerMutation int

const (
	finalizerAdd finalizerMutation = iota
	finalizerRemove
)

func mutateFinalizer(w http.ResponseWriter, r *http.Request, optimizer *performance.Optimizer, op finalizerMutation) {
	target, id, name, ok := extractFinalizerInputs(w, r)
	if !ok {
		return
	}

	lock := concurrency.GetLock(id)
	lock.Lock()
	defer lock.Unlock()

	getResult := api.GetCustomResourceByName(id, target.Metadata)
	if getResult.Status != http.StatusOK {
		// A record that is already gone is done, not a failure to retry.
		responseutils.LogAndSendResponse(w, sharedutils.StatusForResult(getResult), response.OperationError,
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
	patchResult := sharedutils.PatchCustomResource(target.Metadata, id, patch)
	// The last finalizer's removal is when the API server drops a deleted record;
	// a list cached while it was terminating would keep showing it until its TTL.
	resourcesshared.InvalidateResourceCaches(optimizer, target.CacheType, id)
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
	body, err := sharedutils.GetSpecFor[cleanupeps.FinalizerRequest](w, r)
	if err != nil {
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
