package insights

import "github.com/telark/rest/base"

const (
	Applications    base.Endpoint = "insights/applications"
	Analyze         base.Endpoint = "insights/applications/{namespace}/{name}/analyze"
	Events          base.Endpoint = "insights/events"
	Runtime         base.Endpoint = "insights/runtime"
	RuntimeValidate base.Endpoint = "insights/runtime/validate"
	RuntimePull     base.Endpoint = "insights/runtime/pull"
	List            base.Endpoint = "insights/get"
	Triage          base.Endpoint = "insights/applications/{namespace}/{name}/insights/{id}/triage"
)
