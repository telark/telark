package k8sclient

import (
	"fmt"
	"sync"

	"github.com/plsyro/data-pkg/errors"
	globalLogger "github.com/plsyro/data-pkg/logger"
	"github.com/plsyro/kcore-pkg/constants"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	logger           = globalLogger.NewCustomLogger(constants.LoggerPrefixK8sManager)
	kubernetesClient *kubernetes.Clientset
	dynamicClient    dynamic.Interface
	config           *rest.Config
	configOnce       sync.Once
	mu               sync.RWMutex
)

func getConfig() (*rest.Config, error) {
	var err error
	configOnce.Do(func() {
		config, err = rest.InClusterConfig()
		if err != nil {
			logger.Error(fmt.Sprintf(string(errors.ErrK8sCreateConfig), err))
		}
	})
	return config, err
}

func InitDynamicClient() (dynamic.Interface, error) {
	mu.RLock()
	if dynamicClient != nil {
		defer mu.RUnlock()
		return dynamicClient, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if dynamicClient != nil {
		return dynamicClient, nil
	}

	config, err := getConfig()
	if err != nil {
		return nil, err
	}

	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrK8sSetClient), err)
	}

	return dynamicClient, nil
}

func InitKubernetesClient() (*kubernetes.Clientset, error) {
	mu.RLock()
	if kubernetesClient != nil {
		defer mu.RUnlock()
		return kubernetesClient, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if kubernetesClient != nil {
		return kubernetesClient, nil
	}

	config, err := getConfig()
	if err != nil {
		return nil, err
	}

	kubernetesClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrK8sSetClient), err)
	}

	return kubernetesClient, nil
}

func ResetAllClients() {
	mu.Lock()
	defer mu.Unlock()
	kubernetesClient = nil
	dynamicClient = nil
	config = nil
	configOnce = sync.Once{}
}

func GetLogger() *globalLogger.CustomLogger {
	return logger
}
