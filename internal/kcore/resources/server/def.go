package config

import (
	"github.com/plsyro/kcore-pkg/resources/client"
	"k8s.io/apimachinery/pkg/version"
)

func GetServerVersion() (*version.Info, error) {
	// Set Client
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	// Get Server Version
	serverVersion, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	return serverVersion, nil
}
