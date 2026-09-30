package snapshot

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/response"
	requestutils "github.com/telark/telark/internal/rest/utils/request"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	snapshotexp "github.com/telark/telark/services/exporter/internal/exporters/snapshot"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateSnapshot() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			sharedutils.LogByStatusAndSend(
				w,
				http.StatusUnprocessableEntity,
				response.OperationUnprocessed,
				fmt.Sprintf(string(errors.ErrRestParseRequestBody), err),
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

		if !authz.GuardSnapshotManifestView(w, r) {
			return
		}

		scope := r.URL.Query().Get(constants.ScopeParam)
		namespace := r.URL.Query().Get(constants.NamespaceParam)
		generation := r.URL.Query().Get(constants.GenerationParam)
		snapshotexp.ReadSnapshot(w, id, scope, namespace, generation, redactFor(r))
	}
}

// Only a peer holding the service token sees Secret values; a missing identity
// is treated like a session, so nothing leaks by default.
func redactFor(r *http.Request) bool {
	identity, _ := xauthz.FromContext(r.Context())
	return !identity.Internal
}

func GetSnapshotManifest() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		if !authz.GuardSnapshotView(w, r) {
			return
		}

		scope := r.URL.Query().Get(constants.ScopeParam)
		namespace := r.URL.Query().Get(constants.NamespaceParam)
		generation := r.URL.Query().Get(constants.GenerationParam)
		snapshotexp.ReadSnapshotManifestWithAccept(w, id, scope, namespace, generation, strings.TrimSpace(r.Header.Get("Accept")), redactFor(r))
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
