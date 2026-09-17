package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
	restsnapshot "github.com/telark/rest/clients/snapshots"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"sigs.k8s.io/yaml"
)

var lg = constants.GetLogger(constants.PrefixMain)

func CreateSnapshot(w http.ResponseWriter, body map[string]any) {
	snap, err := snaputil.ParseCreateSnapshotRequest(body)
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, nil)
		return
	}
	namespaced, valid := validateCreateScope(w, snap)
	if !valid {
		return
	}

	path, ready := prepareSnapshotPath(w, snap, namespaced)
	if !ready {
		return
	}

	if err := snaputil.WriteSnapshotJSON(path, snap.ID, body); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(constants.ErrSnapshotWriteFailed),
			nil,
			err,
		)
		return
	}

	if retErr := snaputil.ApplyRetentionPolicy(
		envmanager.GetSnapshotsPath(),
		snap.Scope,
		snap.ID,
		snap.Namespace,
		namespaced,
		envmanager.GetSnapshotsMaxVersions(),
	); retErr != nil {
		lg.Warn(fmt.Sprintf(string(constants.ErrSnapshotRetentionPolicyFailed),
			snap.Scope,
			snap.ID,
			retErr))
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.InfSnapshotCreateSuccessful),
		createSnapshotResponse(snap, namespaced),
		nil,
	)
}

func validateCreateScope(w http.ResponseWriter, snap *restsnapshot.CreateSnapshotPayload) (namespaced bool, ok bool) {
	scopes := envmanager.GetSnapshotScopes()
	if !envmanager.IsScopeValid(scopes, snap.Scope) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrSnapshotScopeNotRegistered), snap.Scope, snaputil.RegisteredScopesText(scopes)),
			nil,
			nil,
		)
		return false, false
	}
	namespaced = envmanager.IsScopeNamespaced(scopes, snap.Scope)
	if namespaced && strings.TrimSpace(snap.Namespace) == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrSnapshotNamespaceRequired), snap.Scope),
			nil,
			nil,
		)
		return false, false
	}

	return namespaced, true
}

func prepareSnapshotPath(w http.ResponseWriter, snap *restsnapshot.CreateSnapshotPayload, namespaced bool) (string, bool) {
	dir := snaputil.BuildSnapshotDir(envmanager.GetSnapshotsPath(), snap.Scope, snap.ID, snap.Namespace, namespaced)
	if err := os.MkdirAll(dir, constants.SnapshotDirPerm); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(constants.ErrSnapshotWriteFailed),
			nil,
			err,
		)
		return constants.EmptyString, false
	}

	path := snaputil.BuildSnapshotPath(
		envmanager.GetSnapshotsPath(),
		snap.Scope,
		snap.ID,
		snap.Namespace,
		snap.Generation,
		namespaced,
	)
	return path, true
}

func createSnapshotResponse(snap *restsnapshot.CreateSnapshotPayload, namespaced bool) map[string]any {
	return map[string]any{
		constants.FieldID:         snap.ID,
		constants.FieldScope:      snap.Scope,
		constants.FieldNamespace:  snap.Namespace,
		constants.FieldGeneration: snap.Generation,
		"path":                    snaputil.APISnapshotPath(snap.Scope, snap.ID, snap.Namespace, snap.Generation, namespaced),
	}
}

func ReadSnapshot(w http.ResponseWriter, id string, scope string, namespace string, generation string) {
	target, ok := validateAndResolvePath(w, id, scope, namespace, generation)
	if !ok {
		return
	}

	data, ok := readSnapshotData(w, id, target.Path)
	if !ok {
		return
	}

	fileSize := snaputil.FormattedFileSize(target.Path, id)
	pvcAvailable, pvcTotal, pvcUsedPercent := snaputil.GetStorageInfo()
	manifest := data[constants.FieldManifest]

	resp := map[string]any{
		constants.FieldID:         id,
		constants.FieldScope:      scope,
		constants.FieldNamespace:  target.Namespace,
		constants.FieldGeneration: target.Generation,
		"fileSize":                fileSize,
		"pvcAvailable":            pvcAvailable,
		"pvcTotal":                pvcTotal,
		"pvcUsedPercent":          pvcUsedPercent,
		constants.FieldManifest:   manifest,
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.InfSnapshotRetrievalSuccessful),
		resp,
		nil,
	)
}

func RemoveSnapshot(w http.ResponseWriter, id string, scope string, namespace string, generation string) {
	if !requireExplicitGeneration(w, generation) {
		return
	}
	target, ok := validateAndResolvePath(w, id, scope, namespace, generation)
	if !ok {
		return
	}
	if !ensureWithinSnapshotsBase(w, id, target.Path) {
		return
	}
	if !removeSnapshotFile(w, id, target.Path) {
		return
	}
	sendSnapshotDeleted(w, id, scope, target)
}

// An empty generation resolves to "latest" on the read paths.
func requireExplicitGeneration(w http.ResponseWriter, generation string) bool {
	if strings.TrimSpace(generation) != constants.EmptyString {
		return true
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusBadRequest,
		response.OperationUnprocessed,
		string(constants.ErrSnapshotGenerationForDelete),
		nil,
		nil,
	)
	return false
}

// namespace reaches BuildSnapshotDir unvalidated, so it can traverse outside the base.
func ensureWithinSnapshotsBase(w http.ResponseWriter, id string, path string) bool {
	base := envmanager.GetSnapshotsPath()
	if snaputil.IsWithinBase(path, base) {
		return true
	}
	lg.Error(fmt.Sprintf(string(constants.ErrSnapshotPathOutsideBaseContext), id, path, base))
	responseutils.LogAndSendResponse(
		w,
		http.StatusBadRequest,
		response.OperationUnprocessed,
		string(constants.ErrSnapshotPathOutsideBase),
		nil,
		nil,
	)
	return false
}

func removeSnapshotFile(w http.ResponseWriter, id string, path string) bool {
	err := os.Remove(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			string(constants.ErrSnapshotNotFound),
			nil,
			nil,
		)
		return false
	}
	lg.Error(fmt.Sprintf(string(constants.ErrSnapshotDeleteContext), id, path, err))
	responseutils.LogAndSendResponse(
		w,
		http.StatusInternalServerError,
		response.OperationError,
		string(constants.ErrSnapshotDeleteFailed),
		nil,
		err,
	)
	return false
}

func sendSnapshotDeleted(w http.ResponseWriter, id string, scope string, target *snaputil.SnapshotTarget) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.InfSnapshotDeleteSuccessful),
		map[string]any{
			constants.FieldID:         id,
			constants.FieldScope:      scope,
			constants.FieldNamespace:  target.Namespace,
			constants.FieldGeneration: target.Generation,
		},
		nil,
	)
}

func ReadSnapshotManifest(w http.ResponseWriter, id string, scope string, namespace string, generation string) {
	ReadSnapshotManifestWithAccept(w, id, scope, namespace, generation, constants.EmptyString)
}

func ReadSnapshotManifestWithAccept(
	w http.ResponseWriter,
	id string,
	scope string,
	namespace string,
	generation string,
	accept string,
) {
	target, ok := validateAndResolveManifestPath(w, id, scope, namespace, generation)
	if !ok {
		return
	}

	data, ok := readSnapshotDataForManifest(w, id, scope, target.Path)
	if !ok {
		return
	}

	items, ok := snaputil.BuildKubernetesItems(data)
	if !ok {
		sendManifestError(
			w,
			http.StatusInternalServerError,
			string(constants.OperationInternalServerError),
			string(constants.ErrSnapshotManifestBuildFailed),
			nil,
		)
		return
	}

	items = snaputil.SanitizeManifest(items)

	if wantsYAML(accept) {
		writeYAMLManifest(w, id, target.Generation, items)
		return
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.Header().Set(
		constants.HeaderContentDisposition,
		snaputil.ContentDispositionFilename(id, target.Generation),
	)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(items); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotDecodeContext), id, target.Path, err))
	}
}

func wantsYAML(accept string) bool {
	accept = strings.ToLower(strings.TrimSpace(accept))
	return strings.Contains(accept, "yaml") || strings.Contains(accept, "yml")
}

const lastIndexOffset = 1

func writeYAMLManifest(w http.ResponseWriter, id string, generation int, items []map[string]any) {
	var buf bytes.Buffer
	for i, item := range items {
		if i > constants.DefaultInitValue {
			buf.WriteString("---\n")
		}
		out, err := yaml.Marshal(item)
		if err != nil {
			sendManifestError(
				w,
				http.StatusInternalServerError,
				string(constants.OperationInternalServerError),
				string(constants.ErrSnapshotManifestBuildFailed),
				err,
			)
			return
		}
		buf.Write(out)
		if len(out) > constants.DefaultInitValue && out[len(out)-lastIndexOffset] != '\n' {
			buf.WriteByte('\n')
		}
	}

	w.Header().Set(constants.HeaderContentType, "application/yaml")
	w.Header().Set(
		constants.HeaderContentDisposition,
		snaputil.ContentDispositionFilename(id, generation),
	)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(buf.Bytes()); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotDecodeContext), id, "yaml", err))
	}
}

func validateAndResolvePath(w http.ResponseWriter, id string, scope string, namespace string, generation string) (*snaputil.SnapshotTarget, bool) {
	target, err := snaputil.ResolveTarget(id, scope, namespace, generation)
	if err != nil {
		handleSnapshotResolveError(w, id, scope, namespace, generation, err)
		return nil, false
	}
	return target, true
}

func readSnapshotData(w http.ResponseWriter, id string, path string) (map[string]any, bool) {
	data, err := snaputil.LoadSnapshotData(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrSnapshotNotFound), nil, nil)
			return nil, false
		}
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotReadContext), id, path, err))
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(constants.ErrSnapshotReadFailed),
			nil,
			err,
		)
		return nil, false
	}

	return data, true
}

func validateAndResolveManifestPath(
	w http.ResponseWriter,
	id string,
	scope string,
	namespace string,
	generation string,
) (*snaputil.SnapshotTarget, bool) {
	target, err := snaputil.ResolveTarget(id, scope, namespace, generation)
	if err != nil {
		handleManifestResolveError(w, id, scope, namespace, generation, err)
		return nil, false
	}
	return target, true
}

func readSnapshotDataForManifest(w http.ResponseWriter, id string, scope string, path string) (map[string]any, bool) {
	data, err := snaputil.LoadSnapshotData(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			sendManifestError(
				w,
				http.StatusNotFound,
				string(constants.OperationNotFound),
				fmt.Sprintf(string(constants.ErrSnapshotNotFoundByIDScope), id, scope),
				nil,
			)
			return nil, false
		}

		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotReadContext), id, path, err))
		sendManifestError(
			w,
			http.StatusInternalServerError,
			string(constants.OperationInternalServerError),
			string(constants.ErrSnapshotManifestBuildFailed),
			err,
		)
		return nil, false
	}

	return data, true
}

func sendManifestError(w http.ResponseWriter, status int, operation string, message string, err error) {
	responseutils.LogAndSendResponse(w, status, response.OperationStatus(operation), message, nil, err)
}

func handleSnapshotResolveError(
	w http.ResponseWriter,
	id string,
	scope string,
	namespace string,
	generation string,
	err error,
) {
	if errors.Is(err, snaputil.ErrLatestNotFound) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			snaputil.SnapshotNotFoundMessage(id, scope, namespace, generation),
			nil,
			nil,
		)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, nil)
}

func handleManifestResolveError(
	w http.ResponseWriter,
	id string,
	scope string,
	namespace string,
	generation string,
	err error,
) {
	if errors.Is(err, snaputil.ErrLatestNotFound) {
		sendManifestError(
			w,
			http.StatusNotFound,
			string(constants.OperationNotFound),
			snaputil.SnapshotNotFoundMessage(id, scope, namespace, generation),
			nil,
		)
		return
	}
	sendManifestError(w, http.StatusBadRequest, string(constants.OperationBadRequest), err.Error(), nil)
}

func ReadSnapshotInfos(w http.ResponseWriter) {
	infos := snaputil.BuildSnapshotInfos()
	resp := map[string]any{
		constants.FieldTotalPVCSpace: map[string]any{
			constants.FieldBytes:     infos.TotalPVCSpace.Bytes,
			constants.FieldKilobytes: infos.TotalPVCSpace.KB,
			constants.FieldMegabytes: infos.TotalPVCSpace.MB,
		},
		constants.FieldConsumedSpace: map[string]any{
			constants.FieldBytes:     infos.ConsumedSpace.Bytes,
			constants.FieldKilobytes: infos.ConsumedSpace.KB,
			constants.FieldMegabytes: infos.ConsumedSpace.MB,
			constants.FieldPercent:   infos.ConsumedSpace.Percent,
		},
		constants.FieldAvailableSpace: map[string]any{
			constants.FieldBytes:     infos.AvailableSpace.Bytes,
			constants.FieldKilobytes: infos.AvailableSpace.KB,
			constants.FieldMegabytes: infos.AvailableSpace.MB,
			constants.FieldPercent:   infos.AvailableSpace.Percent,
		},
		constants.FieldTotalSnapshots: infos.TotalSnapshots,
		constants.FieldSnapshotsPath:  envmanager.GetSnapshotsPath(),
		constants.FieldSnapshotScopes: registeredScopeNames(),
		constants.FieldPVCName:        envmanager.GetSnapshotsPVCName(),
		constants.FieldPVCNamespace:   envmanager.GetSnapshotsPVCNamespace(),
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.InfSnapshotInfosRetrievalSuccessful),
		resp,
		nil,
	)
}

func registeredScopeNames() []string {
	scopes := envmanager.GetSnapshotScopes()
	names := make([]string, constants.DefaultInitValue, len(scopes))
	for _, scope := range scopes {
		names = append(names, scope.Name)
	}
	return names
}

// RemoveSnapshotFiles deletes the files behind an application's snapshot
// references and prunes the id directories they leave empty.
func RemoveSnapshotFiles(paths []string) {
	base := envmanager.GetSnapshotsPath()
	for _, path := range paths {
		id := filepath.Base(filepath.Dir(filepath.Dir(path)))
		if !snaputil.IsWithinBase(path, base) {
			lg.Error(fmt.Sprintf(string(constants.ErrSnapshotPathOutsideBaseContext), id, path, base))
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			lg.Error(fmt.Sprintf(string(constants.ErrSnapshotDeleteContext), id, path, err))
			continue
		}
		_ = os.Remove(filepath.Dir(path))
		_ = os.Remove(filepath.Dir(filepath.Dir(path)))
	}
}
