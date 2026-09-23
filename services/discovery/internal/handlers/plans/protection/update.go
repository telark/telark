package protection

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
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
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.PrepareProtectionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf(string(protection.ErrRequestBody), err)
		respondError(w, http.StatusBadRequest, dataerrors.Error(msg), err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanDeployTimeout)
	defer cancel()

	plan, err := update.Run(ctx, buildUpdateDeps(svc), userID, planID, &req)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessUpdateRes),
		plan,
		nil,
	)
}

func buildUpdateDeps(svc *protection.Service) update.Deps {
	return update.Deps{
		Applier:        svc.Applier(),
		Exporter:       svc.Exporter(),
		ResolveApps:    svc.ResolveApps(),
		ListNamespaces: svc.ListNamespaces(),
		Logger:         svc.AppLogger(),
		Clock:          svc.Clock,
		StampHealth:    svc.StampFirstHealth,
	}
}
