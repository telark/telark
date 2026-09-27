package protection

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	plansmd "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/data/plans"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/exporter/internal/utils/artifact"
	"github.com/telark/exporter/internal/utils/concurrency"
	plansutils "github.com/telark/exporter/internal/utils/plans/protection"
	reportsutil "github.com/telark/exporter/internal/utils/reports"
	resourcesutils "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

var (
	lg        = constants.GetLogger(constants.PrefixMain)
	listMutex sync.Mutex
)

func CreatePlan() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetHeader(w, r, constants.HeaderUserID)
		if err != nil {
			return
		}

		body, err := sharedutils.GetSpecFor[plans.ProtectionPlan](w, r)
		if err != nil {
			return
		}

		if !authz.GuardPlanLifecycle(w, r, body) {
			return
		}

		plan, err := sharedutils.ExtractStructFromBody[plans.ProtectionPlan](body)
		if err != nil {
			respondBadRequest(w, err)
			return
		}

		if err := plansutils.Validate(plan); err != nil {
			respondBadRequest(w, err)
			return
		}

		plansutils.ApplyCreateAudit(plan, userID)

		spec, err := sharedutils.StructToSpecMap(plan)
		if err != nil {
			responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
			return
		}

		lock := concurrency.GetLock(plan.ID)
		lock.Lock()
		defer lock.Unlock()

		generics.GenericCreateCustomResource(w, plansmd.ProtectionPlanMetadata, plan.ID, spec)
	}
}

func GetPlanByID() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		planID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		resource, ok := plansutils.FindByIDOrRespond(w, planID)
		if !ok {
			return
		}
		resourcesutils.SendFilteredResourceResponse(w, resource)
	}
}

func ListPlans() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		listMutex.Lock()
		defer listMutex.Unlock()
		generics.GenericListCustomResources(w, plansmd.ProtectionPlanMetadata)
	}
}

func PatchPlanByID() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := sharedutils.GetHeader(w, r, constants.HeaderUserID)
		if err != nil {
			return
		}

		planID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		if _, ok := plansutils.FindByIDOrRespond(w, planID); !ok {
			return
		}

		body, err := sharedutils.GetSpecFor[plans.ProtectionPlan](w, r)
		if err != nil {
			return
		}

		if !authz.GuardPlanLifecycle(w, r, body) {
			return
		}

		if err := plansutils.ValidatePatchScope(body); err != nil {
			respondBadRequest(w, err)
			return
		}

		plansutils.ApplyPatchAudit(body, userID)

		patchData := map[string]any{constants.SpecField: body}

		lock := concurrency.GetLock(planID)
		lock.Lock()
		defer lock.Unlock()

		generics.GenericPatchCustomResource(w, plansmd.ProtectionPlanMetadata, planID, patchData)
	}
}

func DeletePlanByID() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		planID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		if _, ok := plansutils.FindByIDOrRespond(w, planID); !ok {
			return
		}

		lock := concurrency.GetLock(planID)
		lock.Lock()
		defer lock.Unlock()

		result := api.DeleteCustomResourceByName(planID, plansmd.ProtectionPlanMetadata)
		if result.Status != http.StatusOK {
			errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, planID, result.Error)
			responseutils.LogAndSendResponse(w, result.Status, response.OperationError, errorMsg, nil, result.Error)
			return
		}

		// The periodic reports sweep is the retry for a failed removal.
		root := envmanager.GetReportsPath()
		if rerr := reportsutil.RemovePlanReports(root, planID); rerr != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrReportWriteFailed), artifact.StageNone, planID, reportsutil.PlanDir(root, planID), rerr))
		}

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), planID, plansmd.ProtectionPlanMetadata.Kind)
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, msg, nil, nil)
	}
}

func respondBadRequest(w http.ResponseWriter, err error) {
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
}
