package application

import (
	"net/http"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/kcore/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListApplicationSummaries(w http.ResponseWriter) {
	SendApplicationSummaries(w, api.ListCustomResources(metadata.ApplicationAsResourceMetadata))
}

// The list is decoded fresh per call, so pruning in place touches nothing shared.
func SendApplicationSummaries(w http.ResponseWriter, result shared.KubernetesAPIData) {
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
	if list, ok := filtered.(*unstructured.UnstructuredList); ok {
		for i := range list.Items {
			pruneSummary(list.Items[i].Object)
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

func pruneSummary(spec map[string]any) {
	delete(spec, constants.FieldResources)
	delete(spec, constants.FieldSnapshots)
	delete(spec, constants.FieldRollbacks)
	if metrics, ok := spec[constants.FieldMetrics].(map[string]any); ok {
		delete(metrics, constants.FieldWorkloads)
	}
	history, ok := spec[constants.FieldHistory].(map[string]any)
	if !ok {
		return
	}
	changeLog, isList := history[constants.FieldChangeLog].([]any)
	if !isList {
		return
	}
	for _, raw := range changeLog {
		if entry, isMap := raw.(map[string]any); isMap {
			delete(entry, constants.FieldChanges)
		}
	}
}
