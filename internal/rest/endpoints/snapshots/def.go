package snapshots

import "github.com/telark/rest/base"

const (
	CreateSnapshot      base.Endpoint = "snapshots/create"
	GetSnapshot         base.Endpoint = "snapshots/{id}/get"
	GetSnapshotManifest base.Endpoint = "snapshots/{id}/manifest"
	GetSnapshotInfos    base.Endpoint = "snapshots/infos"
	DeleteSnapshot      base.Endpoint = "snapshots/{id}/delete"
)
