package namespaces

import (
	"net/http"
	"slices"

	dataerrors "github.com/telark/telark/internal/data/errors"
	kcorecore "github.com/telark/telark/internal/kcore/resources/core"
	"github.com/telark/telark/internal/rest/response"
	discoveryauthz "github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func GetNamespaces(w http.ResponseWriter, r *http.Request) {
	if !discoveryauthz.NamespacesAllowed(r.Context()) {
		response.SendSingleResponse(
			w,
			response.NewGenericResponse(
				http.StatusForbidden,
				response.OperationForbidden,
				nil,
				string(dataerrors.ErrAuthzForbidden),
			),
		)
		return
	}

	list, err := kcorecore.GetAllNamespaces()
	if err != nil {
		response.SendSingleResponse(
			w,
			response.NewGenericResponse(
				http.StatusInternalServerError,
				response.OperationUnprocessed,
				nil,
				string(constants.ErrInternalServerError),
			),
		)
		return
	}

	namespaces := make([]string, constants.DefaultInitValue, len(list))
	for i := range list {
		name := list[i].Name
		namespaces = append(namespaces, name)
	}

	slices.Sort(namespaces)

	response.SendSingleResponse(
		w,
		response.NewGenericResponse(
			http.StatusOK,
			response.OperationSuccess,
			namespaces,
			string(constants.SuccessNamespacesListed),
		),
	)
}
