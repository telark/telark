package forcesync

import (
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/helpers/logdedup"
)

var (
	lg       = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	logDedup = logdedup.New(logdedup.DefaultWindow)
)
