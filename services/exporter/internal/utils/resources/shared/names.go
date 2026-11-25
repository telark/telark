package shared

import (
	"net/http"
	"slices"
	"strings"

	dataerrors "github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The caller holds the lock until its write lands, so two parallel writes of one
// name cannot both pass. A body that keeps the current name is not checked and takes no lock.
func ReserveNameOrRespond(
	w http.ResponseWriter, md metadata.Metadata, body map[string]any, current string, takenErr dataerrors.Error,
) (unlock func(), ok bool) {
	name, isString := body[constants.FieldName].(string)
	if !isString || strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(current)) {
		return func() {}, true
	}
	lock := concurrency.GetLock(md.Plural + constants.NamesLockSuffix)
	lock.Lock()
	taken, err := nameTaken(md, name)
	if err == nil && !taken {
		return lock.Unlock, true
	}
	lock.Unlock()
	if err != nil {
		responseutils.LogAndSendResponse(w, sharedutils.StatusForK8sError(err), response.OperationError, err.Error(), nil, err)
		return nil, false
	}
	responseutils.SendResponse(w, http.StatusConflict, response.OperationAlreadyExists, string(takenErr), nil)
	return nil, false
}

// Names compare trimmed and case-insensitively, as the dashboard does. A record being
// deleted holds no name, nor does a role kept as Deleted, which the dashboard hides.
func nameTaken(md metadata.Metadata, name string) (bool, error) {
	list, err := sharedutils.ListResources(md, constants.ErrInvalidListOrEmptyItems)
	if err != nil {
		return false, err
	}
	want := strings.TrimSpace(name)
	return slices.ContainsFunc(list.Items, func(item unstructured.Unstructured) bool {
		stored, _, _ := unstructured.NestedString(item.Object, constants.SpecField, constants.FieldName)
		status, _, _ := unstructured.NestedString(item.Object, constants.SpecField, constants.FieldStatus)
		return item.GetDeletionTimestamp() == nil && status != string(roledata.RoleStatusDeleted) &&
			strings.EqualFold(strings.TrimSpace(stored), want)
	}), nil
}
