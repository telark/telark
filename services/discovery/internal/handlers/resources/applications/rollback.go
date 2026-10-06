package applications

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	appsnapshot "github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
	"github.com/telark/telark/services/discovery/internal/helpers/async"
	sharedhelper "github.com/telark/telark/services/discovery/internal/helpers/shared"
)

const (
	rollbackStatusPending = "pending"
	rollbackIDPrefix      = "rbk-"
	rollbackIDSuffixLen   = 6
	notFoundIndex         = -1
)

var localRollbackLocks sync.Map

// A per-request value keeps a TTL-expired holder from releasing the next
// owner's lock; Release only deletes when the stored value matches.
func lockRollback(ctx context.Context, w http.ResponseWriter, name string) (func(), bool) {
	coord, _ := getCoordinationBundle()
	if coord == nil {
		return lockRollbackLocal(w, name)
	}
	key := constants.KeyPrefixLockRollback + name
	value := uuid.NewString()
	acquired, err := coord.Lock.Acquire(ctx, key, value, constants.DefaultLockTTL)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, response.OperationError,
			string(constants.ErrRollbackCoordinationUnavailable), err)
		return nil, false
	}
	if !acquired {
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackInFlight), nil)
		return nil, false
	}
	return func() { _ = coord.Lock.Release(context.Background(), key, value) }, true
}

// Without a bundle (standalone bootstrap, or consumer-group setup failed) the
// routes still serve, so a per-app mutex is what keeps two triggers apart.
func lockRollbackLocal(w http.ResponseWriter, name string) (func(), bool) {
	entry, _ := localRollbackLocks.LoadOrStore(name, &sync.Mutex{})
	mu, ok := entry.(*sync.Mutex)
	if !ok || !mu.TryLock() {
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackInFlight), nil)
		return nil, false
	}
	return mu.Unlock, true
}

func rollbackActive(r applicationmodel.RollbackEntry) bool {
	return r.Status == constants.RollbackStatusPending || r.Status == constants.RollbackStatusInProgress
}

// triggeredBy is the verified caller (X-User-ID), never a body field a caller could forge.
type triggerRollbackBody struct {
	SnapshotGeneration int `json:"snapshotGeneration"`
	// Accepted so older clients still decode under DisallowUnknownFields; the caller comes from the verified identity.
	LegacyTriggeredBy *string `json:"triggeredBy,omitempty"`
}

type triggerRollbackResponse struct {
	RollbackID string `json:"rollbackId"`
	Status     string `json:"status"`
}

func TriggerRollback(w http.ResponseWriter, r *http.Request) {
	name, err := sharedhelper.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		return
	}

	release, ok := lockRollback(r.Context(), w, name)
	if !ok {
		return
	}
	defer release()

	body, ok := decodeTriggerRollbackBody(w, r)
	if !ok {
		return
	}
	userID, ok := rollbackCaller(w, r)
	if !ok {
		return
	}

	exporterClient := clients.NewExporterClient()
	app, ok := getRollbackApp(w, exporterClient, name)
	if !ok {
		return
	}

	// The lock only covers simultaneous calls; a second trigger arriving right
	// after the first must see the pending entry and stop, not append over it.
	if slices.ContainsFunc(app.Rollbacks, rollbackActive) {
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackInFlight), nil)
		return
	}

	snap, ok := validateRollbackTarget(w, app, body.SnapshotGeneration)
	if !ok {
		return
	}

	rollbackID, genErr := newRollbackID()
	if genErr != nil {
		writeError(w, http.StatusInternalServerError, response.OperationError, "failed to generate rollback id", genErr)
		return
	}
	entry := buildRollbackEntry(rollbackID, snap, userID)

	updated := slices.Clone(app.Rollbacks)
	updated = append(updated, entry)

	if err := patchRollbacks(exporterClient, name, updated); err != nil {
		writeError(w, err.status, response.OperationError, err.msg, nil)
		return
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		"rollback intent created",
		triggerRollbackResponse{RollbackID: rollbackID, Status: rollbackStatusPending},
		nil,
	)
}

type abortRollbackResponse struct {
	RollbackID string `json:"rollbackId"`
	Status     string `json:"status"`
}

func AbortRollback(w http.ResponseWriter, r *http.Request) {
	name, err := sharedhelper.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		return
	}
	rollbackID, err := sharedhelper.GetPathParam(w, r, constants.RollbackIDPathParam)
	if err != nil {
		return
	}

	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		writeError(w, http.StatusBadRequest, response.OperationError, string(constants.ErrAbortUserRequired), nil)
		return
	}

	release, ok := lockRollback(r.Context(), w, name)
	if !ok {
		return
	}
	defer release()

	exporterClient := clients.NewExporterClient()
	app, ok := getRollbackApp(w, exporterClient, name)
	if !ok {
		return
	}

	idx, abortable := resolveAbortablePending(w, app.Rollbacks, rollbackID)
	if !abortable {
		return
	}

	updated := slices.Clone(app.Rollbacks)
	now := time.Now().UTC()
	updated[idx].Status = constants.RollbackStatusAborted
	updated[idx].CompletedAt = &now
	updated[idx].Error = fmt.Sprintf("%s%s", constants.RollbackAbortedByPrefix, userID)

	if perr := patchRollbacks(exporterClient, name, updated); perr != nil {
		writeError(w, perr.status, response.OperationError, perr.msg, nil)
		return
	}

	emitRollbackAborted(&updated[idx], name, userID)

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.MsgRollbackAborted),
		abortRollbackResponse{RollbackID: rollbackID, Status: constants.RollbackStatusAborted},
		nil,
	)
}

func resolveAbortablePending(
	w http.ResponseWriter,
	rollbacks []applicationmodel.RollbackEntry,
	rollbackID string,
) (int, bool) {
	idx := findRollbackIndexByID(rollbacks, rollbackID)
	if idx == notFoundIndex {
		writeError(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrRollbackNotFound), nil)
		return notFoundIndex, false
	}
	switch rollbacks[idx].Status {
	case constants.RollbackStatusPending:
		return idx, true
	case constants.RollbackStatusInProgress:
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackNotPending), nil)
		return notFoundIndex, false
	default:
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackTerminal), nil)
		return notFoundIndex, false
	}
}

func emitRollbackAborted(entry *applicationmodel.RollbackEntry, appName, abortedBy string) {
	if entry == nil {
		return
	}
	notifier := clients.NewNotificationClient()
	n := notifclient.Notification{
		UserID:   abortedBy,
		Type:     notifclient.TypeRollbackCompleted,
		Title:    string(constants.NotifRollbackAbortedTitle),
		Message:  fmt.Sprintf(string(constants.NotifRollbackAbortedFormat), appName, entry.TargetGeneration),
		Severity: notifclient.SeverityWarning,
		Metadata: map[string]any{
			notifclient.MetaKeyTargetID:        entry.ID,
			notifclient.MetaKeyApplicationID:   appName,
			notifclient.MetaKeyApplicationName: appName,
			notifclient.MetaKeyStatus:          notifclient.RollbackStatusAborted,
		},
	}
	async.Dispatch(func(ctx context.Context) {
		_ = notifier.Emit(ctx, n)
	})
}

func findRollbackIndexByID(rollbacks []applicationmodel.RollbackEntry, id string) int {
	for i := range rollbacks {
		if rollbacks[i].ID == id {
			return i
		}
	}
	return notFoundIndex
}

func rollbackCaller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		writeError(w, http.StatusBadRequest, response.OperationError, string(constants.ErrRollbackUserRequired), nil)
		return constants.EmptyString, false
	}
	return userID, true
}

func decodeTriggerRollbackBody(w http.ResponseWriter, r *http.Request) (*triggerRollbackBody, bool) {
	var body triggerRollbackBody
	if err := sharedhelper.DecodeJSONStrict(w, r, &body); err != nil {
		status := http.StatusUnprocessableEntity
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			status = http.StatusRequestEntityTooLarge
		}
		responseutils.SendResponse(
			w,
			status,
			response.OperationError,
			"invalid request body",
			nil,
		)
		return nil, false
	}
	if body.SnapshotGeneration <= constants.DefaultInitValue {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			"snapshotGeneration must be > 0",
			nil,
			nil,
		)
		return nil, false
	}
	return &body, true
}

func buildRollbackEntry(
	rollbackID string,
	snap *applicationmodel.ApplicationSnapshot,
	triggeredBy string,
) applicationmodel.RollbackEntry {
	return applicationmodel.RollbackEntry{
		ID:               rollbackID,
		TargetSnapshotID: snap.ID,
		TargetGeneration: snap.Generation,
		TargetPath:       snap.Path,
		TriggeredBy:      triggeredBy,
		TriggeredAt:      time.Now().UTC(),
		Status:           rollbackStatusPending,
		Namespace:        snap.Namespace,
	}
}

type patchErr struct {
	status int
	msg    string
}

func patchRollbacks(c *clients.ExporterClient, name string, rollbacks []applicationmodel.RollbackEntry) *patchErr {
	// A view key: the exporter routes it to .status, where rollbacks live.
	patchBody := map[string]any{constants.RollbackRollbacksKey: rollbacks}
	if err := c.PatchApplicationByNameOrError(name, patchBody); err != nil {
		return &patchErr{status: http.StatusInternalServerError, msg: err.Error()}
	}
	return nil
}

func writeError(
	w http.ResponseWriter,
	status int,
	op response.OperationStatus,
	msg string,
	err error,
) {
	responseutils.LogAndSendResponse(w, status, op, msg, nil, err)
}

// A missing app is the caller's answer; any other lookup failure is still logged.
func getRollbackApp(w http.ResponseWriter, c *clients.ExporterClient, name string) (*applicationmodel.Application, bool) {
	app, err := c.GetApplicationByNameFresh(name)
	if err == nil && app != nil {
		return app, true
	}
	if errors.Is(err, restshared.ErrNotFound) {
		err = nil
	}
	writeError(w, http.StatusNotFound, response.OperationNotFound, string(constants.MsgApplicationNotFound), err)
	return nil, false
}

// A snapshot stamped with generation N is the pre-image of the change that
// produced N, so the one matching the current generation is the newest valid
// target (undo the latest change); only a future generation has nothing behind it.
func validateRollbackTarget(
	w http.ResponseWriter,
	app *applicationmodel.Application,
	gen int,
) (*applicationmodel.ApplicationSnapshot, bool) {
	if gen > app.History.Generation {
		writeError(w, http.StatusBadRequest, response.OperationError,
			fmt.Sprintf(string(constants.ErrRollbackTargetNotOlder), app.History.Generation), nil)
		return nil, false
	}
	idx := slices.IndexFunc(app.Snapshots, func(s applicationmodel.ApplicationSnapshot) bool {
		return s.Generation == gen
	})
	if idx == notFoundIndex {
		writeError(w, http.StatusBadRequest, response.OperationError,
			fmt.Sprintf(string(constants.ErrRollbackSnapshotMissing), gen), nil)
		return nil, false
	}
	// A set missing one of the app's namespaces restores part of the app and reports success.
	covered := appsnapshot.NamespacesForGeneration(app.Snapshots, gen)
	for i := range app.Namespaces.Items {
		if ns := app.Namespaces.Items[i].Name; ns != constants.EmptyString && !slices.Contains(covered, ns) {
			writeError(w, http.StatusBadRequest, response.OperationError,
				fmt.Sprintf(string(constants.ErrRollbackSnapshotIncomplete), gen, ns), nil)
			return nil, false
		}
	}
	return &app.Snapshots[idx], true
}

func newRollbackID() (string, error) {
	u := uuid.New()
	hexStr := hex.EncodeToString(u[:])
	if len(hexStr) < rollbackIDSuffixLen {
		return constants.EmptyString, errors.New("uuid hex too short")
	}
	return fmt.Sprintf("%s%s", rollbackIDPrefix, hexStr[:rollbackIDSuffixLen]), nil
}
