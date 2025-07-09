package watch

import "github.com/plsyro/rest-pkg/base"

const (
	// Groupers
	WATCH_ALL_GROUPERS base.Endpoint = "resources/groupers/watch"
	WATCH__GROUPER     base.Endpoint = "resources/groupers/{name}/watch"

	// Workloads
	WATCH_ALL_APPS_WORKLOADS    base.Endpoint = "resources/workloads/apps/watch"
	WATCH_ALL_BATCHES_WORKLOADS base.Endpoint = "resources/workloads/batches/watch"
	WATCH_APP_WORKLOAD          base.Endpoint = "resources/workloads/apps/{name}/watch"
	WATCH_BATCH_WORKLOAD        base.Endpoint = "resources/workloads/batches/{name}/watch"

	// Bridges
	WATCH_ALL_BRIDGES base.Endpoint = "resources/bridges/watch"
	WATCH_BRIDGE     base.Endpoint = "resources/bridges/{name}/watch"
)