package workloads

import (
	"fmt"
	"net/http"
	"strings"

	appWokrload "github.com/plsyro/data-pkg/resources/workload/app"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	workloadsEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/workloads"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type Client struct{}

func newGETRequest(url string) (*http.Request, error) {
	return http.NewRequest(http.MethodGet, url, nil)
}

func (WokrloadsClient *Client) GetAppWorkloadByName(name string) (*appWokrload.AppWorkloadAsResource, error) {
	apiEndpoint := strings.Replace(string(workloadsEndpoints.GET_APP_WORKLOAD), "{name}", name, 1)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), "appWokrload", name, err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	appWorkload, err := shared.DoRequest[appWokrload.AppWorkloadAsResource](req)
	if err != nil {
		return nil, err
	}
	return appWorkload, nil
}

func (WokrloadsClient *Client) GetAllAppWorkloads() ([]*appWokrload.AppWorkloadAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, workloadsEndpoints.GET_ALL_APPS_WORKLOADS)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	appWorkloads, err := shared.DoRequestList[*appWokrload.AppWorkloadAsResource](req, "items")
	if err != nil {
		return nil, err
	}
	return appWorkloads, nil
}
