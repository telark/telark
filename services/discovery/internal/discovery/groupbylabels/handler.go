package groupbylabels

import (
	"fmt"
	"net/http"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	kcoregroup "github.com/telark/kcore/resources/group"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GroupAppResourcesByLabels(w http.ResponseWriter, r *http.Request) {
	selector, err := sharedhelper.GetRequiredQueryParam(w, r, constants.SelectorParam)
	if err != nil {
		return
	}

	rawNs, _ := sharedhelper.GetOptionalQueryParam(r, constants.NamespaceParamQuery)
	namespaces := discoveryshared.ParseNamespaceList(rawNs)
	resources, err := kcoregroup.SearchResourcesByLabelOrTextInNamespaces(selector, namespaces)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrFailedGroupByLabels), err),
			nil,
			err,
		)
		return
	}

	inputs := discoveryshared.ToDerivationInputs(resources)
	withGroups, _ := derivation.DeriveGroups(inputs)
	data := BuildGroupByLabelsData(withGroups)

	response.SendSingleResponse(
		w,
		response.NewGenericResponse(
			http.StatusOK,
			response.OperationSuccess,
			data,
			string(constants.SuccessGroupByLabelsListed),
		),
	)
}
