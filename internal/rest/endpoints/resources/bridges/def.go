package resources

import "github.com/plsyro/rest-pkg/base"

const (
	CreateBridge           base.Endpoint = "resources/bridges/create"
	GetAllBridges          base.Endpoint = "resources/bridges/get"
	GetBridge              base.Endpoint = "resources/bridges/{name}/get"
	PatchBridge            base.Endpoint = "resources/bridges/{name}/patch"
	UpdateBridgeSync       base.Endpoint = "resources/bridges/{name}/update/sync"
	UpdateBridgeGlobalData base.Endpoint = "resources/bridges/{name}/update/global"
	DeleteBridge           base.Endpoint = "resources/bridges/{name}/delete"
)
