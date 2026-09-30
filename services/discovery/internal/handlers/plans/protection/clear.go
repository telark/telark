package protection

import (
	"context"
	"net/http"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
)

func Clear(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	if _, ok = requireUser(w, r); !ok {
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(detached(r), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	release, ok := lockPlanDecision(ctx, w, planID)
	if !ok {
		return
	}
	defer release()

	if err := svc.Clear(ctx, planID); err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessDeleteRes, planID),
		nil,
		nil,
	)
}
