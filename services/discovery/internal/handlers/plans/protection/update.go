package protection

import (
	"context"
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Update(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.PrepareProtectionPlanRequest
	if !decodeBody(w, r, &req) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanDeployTimeout)
	defer cancel()

	release, ok := lockPlanDecision(ctx, w, planID)
	if !ok {
		return
	}
	defer release()

	plan, err := update.Run(ctx, buildUpdateDeps(svc), userID, planID, &req)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessUpdateRes, planID),
		plan,
		nil,
	)
}

func buildUpdateDeps(svc *protection.Service) update.Deps {
	return update.Deps{
		Applier:         svc.Applier(),
		Exporter:        svc.Exporter(),
		ResolveApps:     svc.ResolveApps(),
		ListNamespaces:  svc.ListNamespaces(),
		Logger:          svc.AppLogger(),
		Clock:           svc.Clock,
		StampHealth:     svc.StampFirstHealth,
		NotifyApprovers: svc.Notifier().RequestApproval,
		LockName:        svc.LockName,
		Activate:        svc.Activate,
	}
}
