package namespaces

import (
	"net/http"
	"slices"

	"github.com/telark/discovery/internal/constants"
	kcorecore "github.com/telark/kcore/resources/core"
	"github.com/telark/rest/response"
)

func GetNamespaces(w http.ResponseWriter, _ *http.Request) {
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
