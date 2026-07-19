package insights

import (
	"context"
	"net/http"
	"strings"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/cache"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

const (
	appsParam      = "apps"
	appKeySep      = "/"
	appKeyFields   = 2
	namespaceIdx   = 0
	nameIdx        = 1
)

// Response is windowed to the apps the caller names — the visible page, never the
// whole cluster — so the body stays bounded no matter how many apps exist. Results
// are whatever is cached now; pending are still being enriched and appear on a
// later poll.
type Response struct {
	Results map[string]*appresource.Insights `json:"results"`
	Pending []string                         `json:"pending"`
}

func GetApplicationsInsights(w http.ResponseWriter, r *http.Request) {
	keys := parseAppKeys(r.URL.Query().Get(appsParam))

	ctx, cancel := context.WithTimeout(context.Background(), constants.InsightsReadTimeout)
	defer cancel()

	rdb := redishelper.NewRedisClient()
	out := Response{
		Results: make(map[string]*appresource.Insights, len(keys)),
		Pending: make([]string, constants.DefaultInitValue, len(keys)),
	}

	for _, key := range keys {
		namespace, name, ok := splitAppKey(key)
		if !ok {
			continue
		}
		insights, err := cache.GetEnrichment(ctx, rdb, namespace, name)
		if err != nil || insights == nil {
			out.Pending = append(out.Pending, key)
			continue
		}
		out.Results[key] = insights
	}

	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess,
		string(constants.InfoInsightsRead), out, nil)
}

func parseAppKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == constants.EmptyString {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, constants.DefaultInitValue, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != constants.EmptyString {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

func splitAppKey(key string) (namespace, name string, ok bool) {
	parts := strings.SplitN(key, appKeySep, appKeyFields)
	if len(parts) != appKeyFields {
		return constants.EmptyString, constants.EmptyString, false
	}
	return parts[namespaceIdx], parts[nameIdx], true
}
