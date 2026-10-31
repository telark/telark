package protection

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	plansmd "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/kcore/crds/api"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	requestutils "github.com/telark/telark/internal/rest/utils/request"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	"github.com/telark/telark/services/exporter/internal/utils/artifact"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	plansutils "github.com/telark/telark/services/exporter/internal/utils/plans/protection"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
	resourcesutils "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

		body, err := sharedutils.GetSpecFor[plans.ProtectionPlan](w, r, sharedutils.PlanTextFields...)
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

		generics.GenericCreateCustomResource(r.Context(), w, plansmd.ProtectionPlanMetadata, plan.ID, spec)
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

		body, err := sharedutils.GetSpecFor[plans.ProtectionPlan](w, r, sharedutils.PlanTextFields...)
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

		if err := plansutils.ValidatePatchRepeat(body); err != nil {
			respondBadRequest(w, err)
			return
		}

		plansutils.TruncateReason(body)
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

		keepReports, err := requestutils.OptionalBoolQuery(r, planseps.QueryKeepReports)
		if err != nil {
			respondBadRequest(w, err)
			return
		}

		plan, ok := plansutils.FindByIDOrRespond(w, planID)
		if !ok {
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

		// The periodic reports sweep is the retry for a failed removal; reports a failed keep
		// left in place go with that sweep too.
		root := envmanager.GetReportsPath()
		var rerr error
		if keepReports {
			name, _, _ := unstructured.NestedString(plan.Object, constants.SpecField, constants.FieldName)
			rerr = reportsutil.Keep(root, planID, name)
		} else {
			rerr = reportsutil.RemovePlanReports(root, planID)
		}
		if rerr != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrReportWriteFailed), artifact.StageNone, planID, reportsutil.PlanDir(root, planID), rerr))
		}

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), planID, plansmd.ProtectionPlanMetadata.Kind)
		responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, msg, nil, nil)
	}
}

func respondBadRequest(w http.ResponseWriter, err error) {
	responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil)
}
