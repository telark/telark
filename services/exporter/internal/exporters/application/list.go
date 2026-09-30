package application

import (
	"maps"
	"net/http"

	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// A fresh read (Cache-Control: no-cache) or an unsynced informer goes to the
// apiserver; everything else is rendered from the watch cache.
func ListApplications(w http.ResponseWriter, view string, fresh bool) {
	if list, ok := informers.ListApplications(); ok && !fresh {
		SendApplicationList(w, shared.CreateKubernetesAPIData(shared.StatusOK, string(messages.SuccessListRes), list, nil), view)
		return
	}
	SendApplicationList(w, api.ListCustomResources(metadata.ApplicationMetadata), view)
}

func SendApplicationList(w http.ResponseWriter, result shared.KubernetesAPIData, view string) {
	filtered, err := sharedutils.FilterData(result.Data)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationUnprocessed,
			string(globalerrors.ErrFilterRes),
			nil,
			err,
		)
		return
	}
	if list, ok := filtered.(*unstructured.UnstructuredList); ok && view == constants.ViewSummary {
		for i := range list.Items {
			list.Items[i].Object = summaryOf(list.Items[i].Object)
		}
	}
	sharedutils.LogByStatusAndSend(
		w,
		result.Status,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		filtered,
		result.Error,
	)
}

// Copy-on-write: the spec may belong to the informer store, so only the maps
// that lose a key are cloned; the bulky values underneath stay shared.
func summaryOf(spec map[string]any) map[string]any {
	out := maps.Clone(spec)
	delete(out, constants.FieldResources)
	delete(out, constants.FieldSnapshots)
	delete(out, constants.FieldRollbacks)
	if metrics, ok := out[constants.FieldMetrics].(map[string]any); ok {
		pruned := maps.Clone(metrics)
		delete(pruned, constants.FieldWorkloads)
		out[constants.FieldMetrics] = pruned
	}
	history, ok := out[constants.FieldHistory].(map[string]any)
	if !ok {
		return out
	}
	changeLog, isList := history[constants.FieldChangeLog].([]any)
	if !isList {
		return out
	}
	entries := make([]any, len(changeLog))
	for i, raw := range changeLog {
		entries[i] = raw
		if entry, isMap := raw.(map[string]any); isMap {
			pruned := maps.Clone(entry)
			delete(pruned, constants.FieldChanges)
			entries[i] = pruned
		}
	}
	pruned := maps.Clone(history)
	pruned[constants.FieldChangeLog] = entries
	out[constants.FieldHistory] = pruned
	return out
}
