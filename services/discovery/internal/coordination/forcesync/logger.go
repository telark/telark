package forcesync

import (
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/helpers/logdedup"
)

var (
	lg       = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	logDedup = logdedup.New(logdedup.DefaultWindow)
)
