package server

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"k8s.io/apimachinery/pkg/version"
)

func GetServerVersion() (*version.Info, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	serverVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetServerVersion), err))
		return nil, err
	}

	return serverVersion, nil
}
