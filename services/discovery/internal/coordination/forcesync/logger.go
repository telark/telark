package forcesync

import (
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/logdedup"
)

var (
	lg       = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	logDedup = logdedup.New(logdedup.DefaultWindow)
)
