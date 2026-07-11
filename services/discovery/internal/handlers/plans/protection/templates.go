package protection

import (
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/data/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GetTemplates(w http.ResponseWriter, _ *http.Request) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessGetRes),
		plans.Templates,
		nil,
	)
}
