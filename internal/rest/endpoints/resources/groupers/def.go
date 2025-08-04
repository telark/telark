package groupers

import "github.com/plsyro/rest-pkg/base"

const (
	CreateGrouper           base.Endpoint = "resources/groupers/create"
	GetAllGroupers          base.Endpoint = "resources/groupers/get"
	GetGrouper              base.Endpoint = "resources/groupers/{name}/get"
	PatchGrouper            base.Endpoint = "resources/groupers/{name}/patch"
	UpdateGrouperSync       base.Endpoint = "resources/groupers/{name}/update/sync"
	UpdateGrouperGlobalData base.Endpoint = "resources/groupers/{name}/update/global"
	DeleteGrouper           base.Endpoint = "resources/groupers/{name}/delete"
)
