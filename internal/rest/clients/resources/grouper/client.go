package grouper

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/plsyro/data-pkg/resources/grouper"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	grouperEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/groupers"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type Client struct{}

func newGETRequest(url string) (*http.Request, error) {
	return http.NewRequest(http.MethodGet, url, nil)
}

func (GrouperClient *Client) GetGrouperByName(name string) (*grouper.GrouperAsResource, error) {
	apiEndpoint := strings.Replace(string(grouperEndpoints.GET_GROUPER), "{name}", name, 1)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), "grouper", name, err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	grouperObj, err := shared.DoRequest[grouper.GrouperAsResource](req)
	if err != nil {
		return nil, err
	}
	return grouperObj, nil
}

func (GrouperClient *Client) GetAllGroupers() ([]*grouper.GrouperAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, grouperEndpoints.GET_ALL_GROUPERS)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	groupers, err := shared.DoRequestList[*grouper.GrouperAsResource](req, "items")
	if err != nil {
		return nil, err
	}
	return groupers, nil
}
