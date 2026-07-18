package application

type Insights struct {
	Enriched           bool               `json:"enriched"`
	EnrichedAt         *string            `json:"enrichedAt"`
	Confidence         *string            `json:"confidence"`
	Summary            *string            `json:"summary"`
	TechStack          []string           `json:"techStack"`
	Role               *string            `json:"role"`
	Dependencies       []string           `json:"dependencies"`
	Category           *string            `json:"category"` // infrastructure | application | data | messaging | security
	Risks              []Risk             `json:"risks"`
	Suggestions        []Suggestion       `json:"suggestions"`
	ResourceEfficiency ResourceEfficiency `json:"resourceEfficiency"`
	Criticality        Criticality        `json:"criticality"`
	Tags               []string           `json:"tags"`
	RelatedApps        []RelatedApp       `json:"relatedApps"`
	PromptVersion      *string            `json:"promptVersion"`
}

type RelatedApp struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Risk struct {
	Severity string `json:"severity"` // high | medium | low
	Message  string `json:"message"`
}

type Suggestion struct {
	Priority string `json:"priority"` // high | medium | low
	Message  string `json:"message"`
}

type ResourceEfficiency struct {
	Status string `json:"status"` // over | under | balanced | unknown
	Note   string `json:"note"`
}

type Criticality struct {
	Level  string `json:"level"` // critical | high | medium | low
	Reason string `json:"reason"`
}
