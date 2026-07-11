package application

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	basemetadata "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/application"
	"github.com/telark/exporter/internal/constants"
	sharedexp "github.com/telark/exporter/internal/exporters/shared"
	"github.com/telark/exporter/internal/handlers/resources/shared"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	appclient "github.com/telark/rest/clients/resources/applications"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func CreateApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.CreateResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata, constants.ResourceApplication, nil)
}

func GetApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.GetResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata)
}

func ListApplicationResourcesWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.ListResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata)
}

func PatchApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.PatchResourceWithCacheInvalidation(optimizer, metadata.ApplicationAsResourceMetadata, constants.ResourceApplication, nil)
}

func DeleteApplicationResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return shared.DeleteResourceWithCacheInvalidation(
		optimizer,
		metadata.ApplicationAsResourceMetadata,
		constants.ResourceApplication,
		deleteApplicationAndTriggerCleanup,
	)
}

func deleteApplicationAndTriggerCleanup(
	w http.ResponseWriter,
	r *http.Request,
	md basemetadata.Metadata,
	resourceName string,
) {
	sharedexp.DeleteResource(w, r, md, resourceName)
	go func(appName string) {
		_ = appclient.NewClient().CleanupApplicationByName(appName)
	}(resourceName)
}

func GetRollbacks() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			return
		}
		spec, err := getApplicationSpec(name)
		if err != nil {
			sharedutils.LogAndReturnError(w, http.StatusNotFound, "application not found", err)
			return
		}

		out := slices.Clone(spec.Rollbacks)
		slices.SortFunc(out, func(a, b application.RollbackEntry) int {
			if a.TriggeredAt.After(b.TriggeredAt) {
				return -1
			}
			if a.TriggeredAt.Before(b.TriggeredAt) {
				return 1
			}
			return 0
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
			sharedutils.LogAndReturnError(w, http.StatusNotFound, "application not found", err)
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
	if result.Status != http.StatusOK || result.Error != nil {
		return nil, fmt.Errorf("get crd failed: %w", result.Error)
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
