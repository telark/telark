package config

import (
	"time"

	"github.com/telark/telark/services/discovery/internal/constants"
)

func InsightsIndexRefresh() time.Duration {
	return time.Duration(envInt(constants.EnvInsightsIndexRefreshSec, constants.DefaultInsightsIndexRefreshSec)) * time.Second
}

func InsightsIndexResync() time.Duration {
	return time.Duration(envInt(constants.EnvInsightsIndexResyncSec, constants.DefaultInsightsIndexResyncSec)) * time.Second
}

func InsightsStaleAfter() time.Duration {
	return time.Duration(envInt(constants.EnvInsightsStaleAfterSec, constants.DefaultInsightsStaleAfterSec)) * time.Second
}
