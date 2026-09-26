package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	basemetadata "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/application"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	applicationexp "github.com/telark/exporter/internal/exporters/application"
	sharedexp "github.com/telark/exporter/internal/exporters/shared"
	snapshotexp "github.com/telark/exporter/internal/exporters/snapshot"
	"github.com/telark/exporter/internal/handlers/resources/shared"
	"github.com/telark/exporter/internal/utils/performance"
	applicationutils "github.com/telark/exporter/internal/utils/resources/application"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/base"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var lg = constants.GetLogger(constants.PrefixMain)

func CreateApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.CreateResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata, constants.ResourceApplication)
}

func GetApplicationResourceWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return shared.GetResourceWithCacheInvalidation(metadata.ApplicationAsResourceMetadata)
}

func ListApplicationResourcesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fresh := r.Header.Get(restconstants.HeaderCacheControl) == restconstants.CacheControlNoCache
		applicationexp.ListApplications(w, r.URL.Query().Get(constants.ViewParam), fresh)
	}
}

func PatchApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	patch := shared.PatchResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata, constants.ResourceApplication)
	return func(w http.ResponseWriter, r *http.Request) {
		if !guardEditableFields(w, r) {
			return
		}
		guardHistoryRegression(r)
		patch(w, r)
	}
}

// An unparsable body is left to the downstream handler, which reports it.
func guardEditableFields(w http.ResponseWriter, r *http.Request) bool {
	patch, target, ok := readPatchTarget(r)
	if !ok {
		return true
	}
	if !authz.GuardApplicationPatch(w, r, patch, target) {
		return false
	}
	if spec, lifted := applicationutils.LiftEditableFields(patch); lifted {
		target = spec
		replaceRequestBody(r, patch)
	}
	if err := applicationutils.ValidateEditableFields(target); err != nil {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return false
	}
	return true
}

// guardHistoryRegression drops history and snapshots from a patch that is behind
// the stored generation: a publisher that lost sight of the CR must never reset it.
func guardHistoryRegression(r *http.Request) {
	patch, target, ok := readPatchTarget(r)
	if !ok {
		return
	}
	incomingHistory, ok := target[constants.FieldHistory].(map[string]any)
	if !ok {
		return
	}
	name := sharedutils.ExtractResourceNameFromRequest(r)
	stored, err := getApplicationSpec(name)
	if err != nil || stored == nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnApplicationHistoryGuardSkipped), name, err))
		return
	}
	incomingGen, regressed := historyRegressed(incomingHistory, stored.History)
	if !regressed {
		return
	}
	orphans := droppedSnapshotPaths(target, stored)
	delete(target, constants.FieldHistory)
	delete(target, constants.FieldSnapshots)
	lg.Warn(fmt.Sprintf(
		string(constants.WarnApplicationHistoryRegressionRejected), name, incomingGen, stored.History.Generation,
	))
	replaceRequestBody(r, patch)
	snapshotexp.RemoveSnapshotFiles(orphans)
}

// The dropped refs will never be stored, so their files would leak; an entry the
// stored CR already references is kept because the equal-generation branch can
// legitimately echo stored snapshots.
func droppedSnapshotPaths(target map[string]any, stored *application.Application) []string {
	incoming, isList := target[constants.FieldSnapshots].([]any)
	if !isList {
		return nil
	}
	orphans := make([]string, constants.DefaultInitValue, len(incoming))
	for _, raw := range incoming {
		entry, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		path, isString := entry[constants.FieldPath].(string)
		if !isString || path == constants.EmptyString {
			continue
		}
		if slices.ContainsFunc(stored.Snapshots, func(s application.ApplicationSnapshot) bool { return s.Path == path }) {
			continue
		}
		orphans = append(orphans, path)
	}
	return orphans
}

// readPatchTarget decodes the patch body and returns the map holding the
// application fields: the notifier wraps NATS payloads as {"spec": {...}},
// direct callers patch fields at the top level. The body is restored for the
// downstream handler.
func readPatchTarget(r *http.Request) (patch, target map[string]any, ok bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, base.MaxRequestBodySize))
	if err != nil {
		return nil, nil, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if json.Unmarshal(body, &patch) != nil {
		return nil, nil, false
	}
	target = patch
	if spec, isSpec := patch[constants.SpecField].(map[string]any); isSpec {
		target = spec
	}
	return patch, target, true
}

func historyRegressed(incoming map[string]any, stored application.ApplicationHistory) (int, bool) {
	gen, ok := incoming[constants.FieldGeneration].(float64)
	if !ok {
		return constants.DefaultInitValue, false
	}
	var entries []any
	if list, isList := incoming[constants.FieldChangeLog].([]any); isList {
		entries = list
	}
	incomingGen := int(gen)
	if incomingGen < stored.Generation {
		return incomingGen, true
	}
	return incomingGen, incomingGen == stored.Generation && len(entries) < len(stored.ChangeLog)
}

func replaceRequestBody(r *http.Request, patch map[string]any) {
	guarded, err := json.Marshal(patch)
	if err != nil {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(guarded))
	r.ContentLength = int64(len(guarded))
}

func DeleteApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.DeleteResourceWithCacheInvalidation(
		optimizer,
		metadata.ApplicationAsResourceMetadata,
		constants.ResourceApplication,
		deleteApplicationAndSnapshots,
	)
}

func deleteApplicationAndSnapshots(w http.ResponseWriter, md basemetadata.Metadata, resourceName string) {
	spec, _ := getApplicationSpec(resourceName)
	sharedexp.DeleteResource(w, md, resourceName)
	if spec != nil && applicationGone(resourceName) {
		snapshotexp.RemoveSnapshotFiles(snapshotPaths(spec.Snapshots))
	}
}

func applicationGone(name string) bool {
	return k8serrors.IsNotFound(api.GetCustomResourceByName(name, metadata.ApplicationAsResourceMetadata).Error)
}

func snapshotPaths(snaps []application.ApplicationSnapshot) []string {
	out := make([]string, constants.DefaultInitValue, len(snaps))
	for i := range snaps {
		out = append(out, snaps[i].Path)
	}
	return out
}

func GetRollbacks() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			return
		}
		spec, err := getApplicationSpec(name)
		if err != nil {
			sharedutils.LogAndReturnError(w, sharedutils.StatusForError(err, http.StatusNotFound), string(constants.ErrResourceLookupFailed), err)
			return
		}

		out := slices.Clone(spec.Rollbacks)
		slices.SortFunc(out, func(a, b application.RollbackEntry) int {
			return b.TriggeredAt.Compare(a.TriggeredAt)
		})
		if out == nil {
			out = []application.RollbackEntry{}
		}
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, "rollbacks fetched", out, nil)
	}
}

func GetRollback() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			return
		}
		rollbackID, err := sharedutils.GetPathParam(w, r, "rollbackId")
		if err != nil {
			return
		}
		spec, err := getApplicationSpec(name)
		if err != nil {
			sharedutils.LogAndReturnError(w, sharedutils.StatusForError(err, http.StatusNotFound), string(constants.ErrResourceLookupFailed), err)
			return
		}
		for _, rb := range spec.Rollbacks {
			if rb.ID == rollbackID {
				responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, "rollback fetched", rb, nil)
				return
			}
		}
		responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound, "rollback not found", nil, nil)
	}
}

func getApplicationSpec(name string) (*application.Application, error) {
	result := api.GetCustomResourceByName(name, metadata.ApplicationAsResourceMetadata)
	if status := sharedutils.StatusForResult(result); status != http.StatusOK {
		return nil, &sharedutils.UpstreamError{Status: status, Err: result.Error}
	}
	cr, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New("invalid crd type")
	}
	specMap, ok := cr.Object["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("missing spec")
	}
	var spec application.Application
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
