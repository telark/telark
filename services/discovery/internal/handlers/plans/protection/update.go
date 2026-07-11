package protection

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/core/plans/protection"
	"github.com/telark/discovery/core/plans/protection/update"
	"github.com/telark/discovery/helpers/shared"
	kcorecore "github.com/telark/kcore/resources/core"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Update(w http.ResponseWriter, r *http.Request) {
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
		respondError(w, http.StatusBadRequest, dataerrors.Error(err.Error()), err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanDeployTimeout)
	defer cancel()

	plan, err := update.Run(ctx, buildUpdateDeps(), userID, planID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, dataerrors.Error(err.Error()), err)
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

func buildUpdateDeps() update.Deps {
	return update.Deps{
		Applier:        globalService.Applier(),
		Exporter:       globalService.Exporter(),
		ResolveApps:    globalService.ResolveApps(),
		ListNamespaces: listClusterNamespaces,
		Logger:         globalService.AppLogger(),
		Clock:          globalService.Clock,
	}
}

func listClusterNamespaces(_ context.Context) ([]string, error) {
	list, err := kcorecore.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	out := make([]string, constants.DefaultInitValue, len(list))
	for i := range list {
		out = append(out, list[i].Name)
	}
	slices.Sort(out)
	return out, nil
}
