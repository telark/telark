package envs

import (
	"errors"
	"strings"

	dataconstants "github.com/telark/data/constants"
	dataerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
)

func InitServiceToken() (string, error) {
	token := strings.TrimSpace(getEnv(dataconstants.EnvServiceToken))
	if token == constants.EmptyString {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzServiceTokenNotSet))
	}

	return token, nil
}
