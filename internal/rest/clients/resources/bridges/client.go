package bridges

import (
	"fmt"
	"net/http"
	"strings"

	bridgeResource "github.com/plsyro/data-pkg/resources/bridge"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/clients/shared"
	"github.com/plsyro/rest-pkg/constants"
	bridgeEndpoints "github.com/plsyro/rest-pkg/endpoints/resources/bridges"
	requestUtils "github.com/plsyro/rest-pkg/utils/request"
)

type Client struct{}

func newGETRequest(url string) (*http.Request, error) {
	return http.NewRequest(http.MethodGet, url, nil)
}

func (BridgesClient *Client) GetBridgeByName(name string) (*bridgeResource.BridgeAsResource, error) {
	apiEndpoint := strings.Replace(string(bridgeEndpoints.GET_BRIDGE), "{name}", name, 1)
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, base.Endpoint(apiEndpoint))
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL_FOR), "bridge", name, err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	bridge, err := shared.DoRequest[bridgeResource.BridgeAsResource](req)
	if err != nil {
		return nil, err
	}
	return bridge, nil
}

func (BridgesClient *Client) GetAllBridges() ([]*bridgeResource.BridgeAsResource, error) {
	request := requestUtils.CreateGenericRequest(base.GET, base.EXPORTER, base.V1, bridgeEndpoints.GET_ALL_BRIDGES)
	requestURL, err := request.GenerateURL()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_GENERATE_REQUEST_URL), err)
	}

	req, err := newGETRequest(requestURL)
	if err != nil {
		return nil, err
	}

	bridges, err := shared.DoRequestList[*bridgeResource.BridgeAsResource](req, "items")
	if err != nil {
		return nil, err
	}
	return bridges, nil
}
