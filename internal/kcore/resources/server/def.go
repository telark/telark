package server

import (
	"fmt"

	k8sClient "github.com/plsyro/kcore-pkg/client"
	"github.com/plsyro/kcore-pkg/constants"
	"k8s.io/apimachinery/pkg/version"
)

func GetServerVersion() (*version.Info, error) {
	client, err := k8sClient.InitKubernetesClient()
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
