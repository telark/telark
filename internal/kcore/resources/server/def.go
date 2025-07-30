package server

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	k8sClient "github.com/plsyro/kcore-pkg/resources/client"
	"k8s.io/apimachinery/pkg/version"
)

func GetServerVersion() (*version.Info, error) {
	client, err := k8sClient.InitClient()
	if err != nil {
		return nil, err
	}

	serverVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_SERVER_VERSION), err))
		return nil, err
	}

	return serverVersion, nil
}
