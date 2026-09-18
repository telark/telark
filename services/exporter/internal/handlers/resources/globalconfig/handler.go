package globalconfig

import (
	"context"
	"net/http"
	"strings"

	metadata "github.com/telark/data/metadata/resources"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/secrets"
	resourcesutils "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetGlobalConfig() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		out, ok := getOrRespond(w)
		if !ok {
			return
		}
		// A config stored before the key moved into the Secret still carries it,
		// so it is dropped first and re-added only for a caller allowed to see it.
		clearAIKey(out)
		if authz.MayControlAIInsights(r) {
			injectAIKey(r.Context(), out)
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
		if !authz.GuardGlobalConfigPatch(w, r, specPatch) {
			return
		}
		if err := divertAIKey(r.Context(), specPatch); err != nil {
			sharedutils.LogAndReturnError(w, http.StatusInternalServerError, string(constants.ErrAIKeyPersistFailed), err)
			return
		}
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
		clearAIKey(res)
		resourcesutils.SendFilteredResourceResponse(w, res)
	}
}

// The key is only ever served by injectAIKey, so any copy riding along on the
// resource itself is a leftover and is removed before the resource is sent.
func clearAIKey(resource *unstructured.Unstructured) {
	spec, found := resource.Object[constants.SpecField].(map[string]any)
	if !found {
		return
	}

	if ai, found := spec[globalconfigresource.FieldAI].(map[string]any); found {
		delete(ai, globalconfigresource.FieldAPIKey)
	}
}

// The key never reaches the CR: it is pulled out of the patch and written to
// the secret, so a CRD without field-level RBAC cannot expose it.
func divertAIKey(ctx context.Context, specPatch map[string]any) error {
	ai, found := specPatch[globalconfigresource.FieldAI].(map[string]any)
	if !found {
		return nil
	}

	key, found := ai[globalconfigresource.FieldAPIKey].(string)
	if !found {
		return nil
	}
	delete(ai, globalconfigresource.FieldAPIKey)

	store, err := secrets.NewAIKeyStore()
	if err != nil {
		return err
	}

	if key == constants.EmptyString {
		return store.Clear(ctx)
	}
	return store.Set(ctx, key)
}

// A key that cannot be read is reported as unset rather than failing the whole
// config read, which would take the dashboard down with it.
func injectAIKey(ctx context.Context, resource *unstructured.Unstructured) {
	store, err := secrets.NewAIKeyStore()
	if err != nil {
		return
	}

	key, err := store.Get(ctx)
	if err != nil || key == constants.EmptyString {
		return
	}

	spec, found := resource.Object[constants.SpecField].(map[string]any)
	if !found {
		return
	}

	ai, found := spec[globalconfigresource.FieldAI].(map[string]any)
	if !found {
		ai = map[string]any{}
		spec[globalconfigresource.FieldAI] = ai
	}
	ai[globalconfigresource.FieldAPIKey] = key
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
	if status := sharedutils.StatusForResult(result); status != http.StatusOK {
		sharedutils.LogAndReturnError(w, status, string(constants.ErrResourceLookupFailed), result.Error)
		return nil, false
	}
	cr, ok := result.Data.(*unstructured.Unstructured)
	if !ok || cr == nil {
		sharedutils.LogAndReturnError(w, http.StatusInternalServerError, "invalid global config type", nil)
		return nil, false
	}
	return cr, true
}
