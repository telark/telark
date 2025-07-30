package client

import (
	"fmt"
	"sync"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/constants"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	logger = logging.NewCustomLogger(constants.LOGGER_PREFIX_KUBERNETES_RESOURCES)
	client *kubernetes.Clientset
	once   sync.Once
	mu     sync.RWMutex
)

func InitClient() (*kubernetes.Clientset, error) {
	mu.RLock()
	if client != nil {
		defer mu.RUnlock()
		return client, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if client != nil {
		return client, nil
	}

	return initClientOnce()
}

func GetLogger() *logging.CustomLogger {
	return logger
}

func initClientOnce() (*kubernetes.Clientset, error) {
	once.Do(func() {
		var err error
		client, err = createClient()
		if err != nil {
			logger.Error(fmt.Sprintf(string(errors.ERROR_K8S_SET_CLIENT), err))
		}
	})

	return client, nil
}

func createClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", string(errors.ERROR_K8S_CREATE_CONFIG), err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	return client, nil
}

func ResetClient() {
	mu.Lock()
	defer mu.Unlock()
	client = nil
	once = sync.Once{}
}
