package shared

import (
	"fmt"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
)

func GetCachedConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGetConfig), err)
	}
	return cfg, nil
}

func IsError(err error, target errors.Error) bool {
	if err == nil {
		return false
	}
	return err.Error() == string(target)
}
