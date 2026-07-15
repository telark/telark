package snapshot

import (
	"net/http"
	"strings"

	"github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	snapshotexp "github.com/telark/exporter/internal/exporters/snapshot"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateSnapshot() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusUnprocessableEntity,
				response.OperationUnprocessed,
				string(errors.ErrRestParseRequestBody),
				nil,
				err,
			)
			return
		}

		snapshotexp.CreateSnapshot(w, body)
	}
}

func GetSnapshot() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		scope := r.URL.Query().Get(constants.ScopeParam)
		namespace := r.URL.Query().Get(constants.NamespaceParam)
		generation := r.URL.Query().Get(constants.GenerationParam)
		snapshotexp.ReadSnapshot(w, id, scope, namespace, generation)
	}
}

func GetSnapshotManifest() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		scope := r.URL.Query().Get(constants.ScopeParam)
		namespace := r.URL.Query().Get(constants.NamespaceParam)
		generation := r.URL.Query().Get(constants.GenerationParam)
		snapshotexp.ReadSnapshotManifestWithAccept(w, id, scope, namespace, generation, strings.TrimSpace(r.Header.Get("Accept")))
	}
}

func DeleteSnapshot() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		scope := r.URL.Query().Get(constants.ScopeParam)
		namespace := r.URL.Query().Get(constants.NamespaceParam)
		generation := r.URL.Query().Get(constants.GenerationParam)
		snapshotexp.RemoveSnapshot(w, id, scope, namespace, generation)
	}
}

func GetSnapshotInfos() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshotexp.ReadSnapshotInfos(w)
	}
}
