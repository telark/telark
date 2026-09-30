package snapshots

import "github.com/telark/rest/base"

const (
	CreateSnapshot      base.Endpoint = "internal/snapshots"
	GetSnapshot         base.Endpoint = "snapshots/{id}"
	GetSnapshotManifest base.Endpoint = "snapshots/{id}/manifest"
	GetSnapshotInfos    base.Endpoint = "snapshots"
	DeleteSnapshot      base.Endpoint = "snapshots/{id}"
)
