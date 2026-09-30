package config

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/kcore/crds/api"
	kshared "github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/oidctrust"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixMain)

func GetConfig() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		out, ok := getOrRespond(w)
		if !ok {
			return
		}
		jwk, err := oidctrust.ReadJWK()
		if err != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnOIDCTrustMergeFailed), err))
		}
		sendConfigView(w, out, jwk, messages.SuccessGetRes)
	}
}

func PatchConfig() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}
		specPatch := extractSpecPatch(body)
		if !authz.GuardConfigPatch(w, r, specPatch) {
			return
		}
		if !canonicalSpecKeys(w, specPatch) {
			return
		}
		jwk, ok := writeJWK(w, specPatch)
		if !ok {
			return
		}
		patchBody := map[string]any{constants.SpecField: specPatch}
		if metadataPatch, ok := extractMetadataPatch(body); ok {
			patchBody[constants.MetadataField] = metadataPatch
		}
		result := sharedutils.PatchCustomResource(metadata.TelarkConfigMetadata, constants.TelarkConfigResourceName, patchBody)
		if result.Error != nil || result.Status != http.StatusOK {
			respondPatchFailure(w, result)
			return
		}
		res, ok := result.Data.(*unstructured.Unstructured)
		if !ok || res == nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusInternalServerError,
				response.OperationError,
				string(constants.ErrConfigInvalidReply),
				nil,
				nil,
			)
			return
		}
		sendConfigView(w, res, jwk, messages.SuccessUpdateRes)
	}
}

// The Secret is written first: a CR patch naming a trust anchor that failed to
// persist would leave auth trusting nothing.
func writeJWK(w http.ResponseWriter, specPatch map[string]any) (string, bool) {
	jwk, present, err := oidctrust.TakeJWK(specPatch)
	if err != nil {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return constants.EmptyString, false
	}
	if !present {
		stored, readErr := oidctrust.ReadJWK()
		if readErr != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnOIDCTrustMergeFailed), readErr))
		}
		return stored, true
	}
	if err := oidctrust.WriteJWK(jwk); err != nil {
		sharedutils.LogAndReturnError(w, http.StatusInternalServerError, string(constants.ErrConfigPatchFailed), err)
		return constants.EmptyString, false
	}
	return jwk, true
}

func sendConfigView(w http.ResponseWriter, cr *unstructured.Unstructured, jwk string, success messages.Message) {
	out := sharedutils.ToView(cr)
	if out == nil {
		sharedutils.LogAndReturnError(w, http.StatusInternalServerError, string(constants.ErrConfigInvalidType), nil)
		return
	}
	oidctrust.MergeJWK(out, jwk)
	msg := fmt.Sprintf(string(success), cr.GetName(), cr.GetKind())
	responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, msg, out)
}

// A schema rejection is the caller's mistake: 400 naming the fields, without
// the API server's validation text.
func respondPatchFailure(w http.ResponseWriter, result kshared.KubernetesAPIData) {
	if k8serrors.IsInvalid(result.Error) {
		message := fmt.Sprintf(string(constants.ErrConfigInvalidField), InvalidFields(result.Error))
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, message, nil, result.Error)
		return
	}
	status := http.StatusInternalServerError
	if isConflictError(result.Message) {
		status = constants.ConflictStatus
	}
	sharedutils.LogAndReturnError(w, status, string(constants.ErrConfigPatchFailed), result.Error)
}

func InvalidFields(err error) string {
	var apiStatus k8serrors.APIStatus
	if !errors.As(err, &apiStatus) || apiStatus.Status().Details == nil {
		return constants.SpecField
	}
	fields := make([]string, constants.DefaultInitValue, len(apiStatus.Status().Details.Causes))
	for _, cause := range apiStatus.Status().Details.Causes {
		if cause.Field != constants.EmptyString && cause.Field != constants.NilFieldPath {
			fields = append(fields, cause.Field)
		}
	}
	if len(fields) == constants.DefaultInitValue {
		return constants.SpecField
	}
	return strings.Join(slices.Compact(slices.Sorted(slices.Values(fields))), constants.ListSeparator)
}

func extractSpecPatch(body map[string]any) map[string]any {
	specPatch, ok := body[constants.SpecField].(map[string]any)
	if ok && specPatch != nil {
		return specPatch
	}
	return body
}

// An unwrapped body carries its metadata beside the spec fields.
func canonicalSpecKeys(w http.ResponseWriter, specPatch map[string]any) bool {
	keys := maps.Clone(specPatch)
	delete(keys, constants.MetadataField)
	if err := sharedutils.CheckCanonicalKeys[telarkconfig.TelarkConfig](keys); err != nil {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return false
	}
	return true
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
	result := api.GetCustomResourceByName(constants.TelarkConfigResourceName, metadata.TelarkConfigMetadata)
	if status := sharedutils.StatusForResult(result); status != http.StatusOK {
		sharedutils.LogAndReturnError(w, status, string(constants.ErrResourceLookupFailed), result.Error)
		return nil, false
	}
	cr, ok := result.Data.(*unstructured.Unstructured)
	if !ok || cr == nil {
		sharedutils.LogAndReturnError(w, http.StatusInternalServerError, string(constants.ErrConfigInvalidType), nil)
		return nil, false
	}
	return cr, true
}
