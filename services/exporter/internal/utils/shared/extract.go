package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetPathParam(w http.ResponseWriter, r *http.Request, param string) (string, error) {
	return requestutils.PathParam(w, r, param)
}

func GetSpec(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	spec, err := requestutils.ParseRequestBody(r)
	if err != nil {
		LogAndReturnError(w, http.StatusUnprocessableEntity, string(dataerrors.ErrRestParseRequestBody), err)
		return nil, err
	}
	return spec, nil
}

func ExtractResourceNameFromRequestBody(spec map[string]any) string {
	if name, ok := spec["name"].(string); ok {
		return name
	}

	return constants.EmptyString
}

func ExtractMapValue(data map[string]any, key string) (map[string]any, bool) {
	if value, ok := data[key]; ok {
		if mapValue, isMap := value.(map[string]any); isMap {
			return mapValue, true
		}
	}
	return nil, false
}

func ExtractResourceNameFromRequest(r *http.Request) string {
	// Prefer router variables when available
	if vars := mux.Vars(r); vars != nil {
		if name, ok := vars[constants.NameParam]; ok && name != constants.EmptyString {
			return name
		}
	}
	// Fallback to path parsing
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) >= constants.IndexSecondLastElementOffset {
		return pathParts[len(pathParts)-constants.IndexSecondLastElementOffset]
	}
	return constants.EmptyString
}

func LogAndReturnError(w http.ResponseWriter, statusCode int, errorMessage string, err error) {
	message := fmt.Sprintf("%s: %v", errorMessage, err)
	responseutils.LogAndSendResponse(w, statusCode, response.OperationError, message, nil, err)
}

func ListResources(resourceMetadata metadata.Metadata, listFormatErr dataerrors.Error) (*unstructured.UnstructuredList, error) {
	result := api.ListCustomResources(resourceMetadata)
	if result.Error != nil {
		return nil, result.Error
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(listFormatErr))
	}

	return list, nil
}

func RemoveIDFromRequestBody(body map[string]any) {
	if body != nil {
		delete(body, constants.FieldID)
	}
}

func AddCreationDateToRequestBody(body map[string]any) {
	if body != nil {
		if _, exists := body[constants.FieldCreationDate]; !exists {
			body[constants.FieldCreationDate] = time.Now().UTC().Format(time.RFC3339)
		}
	}
}

func ExtractStructFromBody[T any](body map[string]any) (*T, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestMarshalPayload), err)
	}

	var result T
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestUnmarshalRequestBodyToJSON), err)
	}

	return &result, nil
}

func ExtractStructFromBodyIgnoringID[T any](body map[string]any) (*T, error) {
	RemoveIDFromRequestBody(body)
	AddCreationDateToRequestBody(body)
	return ExtractStructFromBody[T](body)
}

func GetHeader(w http.ResponseWriter, r *http.Request, headerName string) (string, error) {
	headerValue := r.Header.Get(headerName)
	if headerValue == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), headerName)
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationUnprocessed, msg, nil, nil)
		return constants.EmptyString, fmt.Errorf("%s", msg)
	}

	return headerValue, nil
}
