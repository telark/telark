package protection

import (
	"net/http"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
)

func GetTemplates(w http.ResponseWriter, _ *http.Request) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		plans.Templates,
		nil,
	)
}
