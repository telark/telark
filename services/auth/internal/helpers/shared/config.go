package shared

import (
	"fmt"
	"sync"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/data/errors"
)

var (
	cachedConfig *config.Config
	configOnce   sync.Once
	configMutex  sync.RWMutex
)

func GetCachedConfig() (*config.Config, error) {
	configMutex.RLock()
	if cachedConfig != nil {
		cfg := cachedConfig
		configMutex.RUnlock()
		return cfg, nil
	}
	configMutex.RUnlock()

	var err error
	configOnce.Do(func() {
		cfg, loadErr := config.GetConfig()
		if loadErr != nil {
			err = fmt.Errorf(string(constants.ErrFailedGetConfig), loadErr)
			return
		}
		configMutex.Lock()
		cachedConfig = cfg
		configMutex.Unlock()
	})

	if err != nil {
		return nil, err
	}

	configMutex.RLock()
	cfg := cachedConfig
	configMutex.RUnlock()
	return cfg, nil
}

func IsError(err error, target errors.Error) bool {
	if err == nil {
		return false
	}
	return err.Error() == string(target)
}
