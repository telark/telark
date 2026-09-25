package insights

const (
	TriggerManual   = "manual"
	TriggerIncident = "incident"
	TriggerRecovery = "recovery"

	RuntimeStateAbsent       = "absent"
	RuntimeStateUnreachable  = "unreachable"
	RuntimeStateModelMissing = "model_missing"
	RuntimeStatePulling      = "pulling"
	RuntimeStateUnsupported  = "unsupported"
	RuntimeStateReady        = "ready"

	ModeFast = "fast"
	ModeDeep = "deep"
)

type Job struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Trigger    string `json:"trigger"`
	Generation int    `json:"generation"`
}

type AnalyzeResponse struct {
	RunID  string `json:"runId"`
	Status string `json:"status"`
}

type RuntimeStatus struct {
	State    string        `json:"state"`
	Model    string        `json:"model"`
	Reason   string        `json:"reason"`
	Mode     string        `json:"mode"`
	AutoPull bool          `json:"autoPull"`
	Pull     *PullProgress `json:"pull,omitempty"`
}

type PullProgress struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	Completed int64  `json:"completed"`
	Total     int64  `json:"total"`
}

type TriageRequest struct {
	Action string `json:"action"`
}

type ValidateModelRequest struct {
	Model string `json:"model"`
}

type ValidateModelResponse struct {
	OK           bool     `json:"ok"`
	Model        string   `json:"model"`
	License      string   `json:"license"`
	Warning      string   `json:"warning"`
	Reason       string   `json:"reason"`
	Capabilities []string `json:"capabilities"`
}
