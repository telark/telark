package cache

import (
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
)

const (
	cachePrefix    = "enrichment"
	inflightSuffix = ":inflight:"
	queueKey       = "enrichment:jobs"
	hasAnyCount    = constants.DefaultInitValue
	firstItemIdx   = constants.DefaultInitValue
)

// DateTime layouts for parsing Python/API enrichment timestamps.
var DateTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999",
	"2006-01-02T15:04:05",
}

// Cache log/error messages.
const (
	MsgRedisClientNil         = "enrichment cache: Redis client is nil, insights will be empty"
	MsgCacheGetFailed         = "enrichment cache GET failed: key=%s err=%v"
	MsgCacheDeserializeFailed = "cache deserialize failed: key=%s err=%v raw=%s"
	MsgCannotParseDatetime    = "cannot parse datetime: %s"
)

// FlexTime parses Python datetime (naive or RFC3339).
type FlexTime struct {
	time.Time
}

// cachedEnrichment matches Python EnrichmentResult; accepts snake_case fallbacks.
type cachedEnrichment struct {
	Summary         string                   `json:"summary"`
	TechStack       []string                 `json:"techStack"`
	Role            string                   `json:"role"`
	Dependencies    []string                 `json:"dependencies"`
	Confidence      string                   `json:"confidence"`
	EnrichedAt      FlexTime                 `json:"enrichedAt"`
	Category        string                   `json:"category"`
	Risks           []string                 `json:"risks"`
	Suggestions     []string                 `json:"suggestions"`
	RelatedApps     []application.RelatedApp `json:"relatedApps"`
	PromptVersion   string                   `json:"promptVersion"`
	TechStackSnake  []string                 `json:"tech_stack"`
	EnrichedAtSnake *string                  `json:"enriched_at"`
}

// AppSignals is the job payload for the Python worker; result is stored at CacheKey(Namespace, Name).
type AppSignals struct {
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Images        []string `json:"images"`
	Ports         []int    `json:"ports"`
	EnvVarKeys    []string `json:"envVarKeys"`
	ResourceKinds []string `json:"resourceKinds"`
	HasIngress    bool     `json:"hasIngress"`
	HasPVC        bool     `json:"hasPVC"`
}
