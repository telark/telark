package k8sclient

import (
	"fmt"
	"sync"

	"github.com/plsyro/data/errors"
	globallogger "github.com/plsyro/data/logger"
	"github.com/plsyro/kcore/constants"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	logger           = globallogger.NewCustomLogger(constants.LoggerPrefixK8sManager)
	kubernetesClient *kubernetes.Clientset
	dynamicClient    dynamic.Interface
	configLoader     func() (*rest.Config, error)
	mu               sync.RWMutex
)

func makeConfigLoader() func() (*rest.Config, error) {
	return sync.OnceValues(func() (*rest.Config, error) {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			logger.Error(fmt.Sprintf(string(errors.ErrK8sCreateConfig), err))
			return nil, err
		}
		return cfg, nil
	})
}

func getConfig() (*rest.Config, error) {
	return configLoader()
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
	configLoader = makeConfigLoader()
}

func GetLogger() *globallogger.CustomLogger {
	return logger
}

func init() {
	configLoader = makeConfigLoader()
}
