package globalconfig

import (
	"net/http"
	"strings"

	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/constants"
	resourcesutils "github.com/telark/exporter/utils/resources/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetGlobalConfig() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		out, ok := getOrRespond(w)
		if !ok {
			return
		}
		resourcesutils.SendFilteredResourceResponse(w, out)
	}
}

func PatchGlobalConfig() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}
		specPatch := extractSpecPatch(body)
		patchBody := map[string]any{constants.SpecField: specPatch}
		if metadataPatch, ok := extractMetadataPatch(body); ok {
			patchBody[constants.MetadataField] = metadataPatch
		}
		result := api.PatchCustomResource(metadata.GlobalConfigMetadata, constants.GlobalConfigResourceName, patchBody)
		if result.Error != nil || result.Status != http.StatusOK {
			status := http.StatusInternalServerError
			if isConflictError(result.Message) {
				status = constants.ConflictStatus
			}
			sharedutils.LogAndReturnError(
				w,
				status,
				"failed to patch global config",
				result.Error,
			)
			return
		}
		res, ok := result.Data.(*unstructured.Unstructured)
		if !ok || res == nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusInternalServerError,
				response.OperationError,
				"invalid global config response type",
				nil,
				nil,
			)
			return
		}
		resourcesutils.SendFilteredResourceResponse(w, res)
	}
}

func extractSpecPatch(body map[string]any) map[string]any {
	specPatch, ok := body[constants.SpecField].(map[string]any)
	if ok && specPatch != nil {
		return specPatch
	}
	return body
}

func extractMetadataPatch(spec map[string]any) (map[string]any, bool) {
	metadataMap, ok := spec[constants.MetadataField].(map[string]any)
	if !ok || metadataMap == nil {
		return nil, false
	}
	resourceVersion, ok := metadataMap[constants.ResourceVersionField].(string)
	if !ok || strings.TrimSpace(resourceVersion) == constants.EmptyString {
		return nil, false
	}
	return map[string]any{
		constants.ResourceVersionField: strings.TrimSpace(resourceVersion),
	}, true
}

func isConflictError(message string) bool {
	return strings.Contains(strings.ToLower(message), constants.ConflictMessageFragment)
}

func getOrRespond(w http.ResponseWriter) (*unstructured.Unstructured, bool) {
	result := api.GetCustomResourceByName(constants.GlobalConfigResourceName, metadata.GlobalConfigMetadata)
	if result.Error != nil || result.Status != http.StatusOK {
		sharedutils.LogAndReturnError(w, http.StatusNotFound, "global config not found", result.Error)
		return nil, false
	}
	cr, ok := result.Data.(*unstructured.Unstructured)
	if !ok || cr == nil {
		sharedutils.LogAndReturnError(w, http.StatusInternalServerError, "invalid global config type", nil)
		return nil, false
	}
	return cr, true
}
