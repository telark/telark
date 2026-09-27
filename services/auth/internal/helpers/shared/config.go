package shared

import (
	"fmt"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/data/errors"
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
