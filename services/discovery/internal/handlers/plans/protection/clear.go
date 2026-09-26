package protection

import (
	"context"
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

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
