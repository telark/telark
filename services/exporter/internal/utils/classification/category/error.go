package category

import (
	"errors"
	"net/http"

	"github.com/telark/exporter/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func handleCategoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrCategoryNotFound) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			string(constants.ErrCategoryNotFound),
			nil,
			nil,
		)
		return
	}

	if errors.Is(err, ErrCategoriesCRDNotFound) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			string(constants.ErrCategoriesCRDNotFound),
			nil,
			nil,
		)
		return
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusInternalServerError,
		response.OperationError,
		err.Error(),
		nil,
		err,
	)
}
