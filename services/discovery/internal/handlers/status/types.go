package status

type Diagnostics struct {
	Degraded bool     `json:"degraded"`
	Reasons  []string `json:"reasons,omitempty"`
}
