package shared

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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

func GetQueryParam(w http.ResponseWriter, r *http.Request, param string) (string, error) {
	value := r.URL.Query().Get(param)
	if value == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), param)
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationUnprocessed, msg, nil, nil)
		return constants.EmptyString, errors.New(msg)
	}
	return value, nil
}

func GetSpec(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	spec, err := requestutils.ParseRequestBody(r)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, requestutils.ErrRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		msg := fmt.Sprintf(string(dataerrors.ErrRestParseRequestBody), err)
		responseutils.LogAndSendResponse(w, status, response.OperationError, msg, nil, err)
		return nil, err
	}
	return spec, nil
}

func ExtractResourceNameFromRequestBody(spec map[string]any) string {
	if name, ok := spec[constants.FieldName].(string); ok {
		return name
	}

	return constants.EmptyString
}

func ExtractResourceNameFromRequest(r *http.Request) string {
	if vars := mux.Vars(r); vars != nil {
		if name, ok := vars[constants.NameParam]; ok && name != constants.EmptyString {
			return name
		}
	}
	// Every by-name route ends with the name, so the last segment is the resource.
	pathParts := strings.Split(r.URL.Path, "/")
	return pathParts[len(pathParts)-constants.IndexLastElementOffset]
}

func LogAndReturnError(w http.ResponseWriter, statusCode int, errorMessage string, err error) {
	message := fmt.Sprintf(constants.LogMessageWithError, errorMessage, err)
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
	if err := CheckCanonicalKeys[T](body); err != nil {
		return nil, err
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestMarshalPayload), err)
	}

	var result T
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf(string(dataerrors.ErrRestUnmarshalRequestBodyToJSON), err)
	}

	return &result, nil
}

func ExtractStructFromBodyIgnoringID[T any](body map[string]any) (*T, error) {
	RemoveIDFromRequestBody(body)
	if _, declared := jsonFields(reflect.TypeFor[T]())[constants.FieldCreationDate]; declared {
		AddCreationDateToRequestBody(body)
	}
	return ExtractStructFromBody[T](body)
}

func GetSpecFor[T any](w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	spec, err := GetSpec(w, r)
	if err != nil {
		return nil, err
	}
	err = CheckCanonicalKeys[T](spec)
	if err == nil {
		err = CheckRefIDs(spec)
	}
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return nil, err
	}
	return spec, nil
}

func GetHeader(w http.ResponseWriter, r *http.Request, headerName string) (string, error) {
	headerValue := r.Header.Get(headerName)
	if headerValue == constants.EmptyString {
		msg := fmt.Sprintf(string(dataerrors.ErrRestRequiredParam), headerName)
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationUnprocessed, msg, nil, nil)
		return constants.EmptyString, errors.New(msg)
	}

	return headerValue, nil
}
