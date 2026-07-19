package cache

import (
	"time"

	"github.com/telark/data/resources/application"
)

const cachePrefix = "enrichment"

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
	Category           string                        `json:"category"`
	Risks              []application.Risk            `json:"risks"`
	Suggestions        []application.Suggestion      `json:"suggestions"`
	ResourceEfficiency application.ResourceEfficiency `json:"resourceEfficiency"`
	Criticality        application.Criticality       `json:"criticality"`
	Tags               []string                      `json:"tags"`
	RelatedApps        []application.RelatedApp      `json:"relatedApps"`
	PromptVersion      string                        `json:"promptVersion"`
	TechStackSnake     []string                      `json:"tech_stack"`
	EnrichedAtSnake    *string                       `json:"enriched_at"`
}

