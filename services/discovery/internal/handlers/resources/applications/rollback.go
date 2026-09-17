package applications

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/async"
	sharedhelper "github.com/telark/discovery/internal/helpers/shared"
	notifclient "github.com/telark/rest/clients/notifications"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

type triggerRollbackBody struct {
	SnapshotGeneration int    `json:"snapshotGeneration"`
	TriggeredBy        string `json:"triggeredBy"`
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

	exporterClient := clients.NewExporterClient()
	app, getErr := exporterClient.GetApplicationByNameFresh(name)
	if getErr != nil || app == nil {
		writeError(w, http.StatusNotFound, response.OperationNotFound, string(constants.MsgApplicationNotFound), getErr)
		return
	}

	// The lock only covers simultaneous calls; a second trigger arriving right
	// after the first must see the pending entry and stop, not append over it.
	if slices.ContainsFunc(app.Rollbacks, rollbackActive) {
		writeError(w, http.StatusConflict, response.OperationError, string(constants.ErrRollbackInFlight), nil)
		return
	}

	snaps, found := findSnapshotsByGeneration(app.Snapshots, body.SnapshotGeneration)
	if !found {
		writeError(w, http.StatusNotFound, response.OperationNotFound, "Snapshot generation not found.", nil)
		return
	}

	rollbackID, genErr := newRollbackID()
	if genErr != nil {
		writeError(w, http.StatusInternalServerError, response.OperationError, "failed to generate rollback id", genErr)
		return
	}
	entry := buildRollbackEntry(rollbackID, &snaps[constants.DefaultInitValue], body.TriggeredBy)

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
	app, getErr := exporterClient.GetApplicationByNameFresh(name)
	if getErr != nil || app == nil {
		writeError(w, http.StatusNotFound, response.OperationNotFound, string(constants.MsgApplicationNotFound), getErr)
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
			notifclient.MetaKeyApplicationID:   entry.ID,
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

func decodeTriggerRollbackBody(w http.ResponseWriter, r *http.Request) (*triggerRollbackBody, bool) {
	var body triggerRollbackBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusUnprocessableEntity,
			response.OperationError,
			"invalid request body",
			nil,
			err,
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
	if body.TriggeredBy == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			"triggeredBy cannot be empty",
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
	patchBody := map[string]any{
		"spec": map[string]any{
			"rollbacks": rollbacks,
		},
	}
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

func findSnapshotsByGeneration(
	snapshots []applicationmodel.ApplicationSnapshot,
	gen int,
) ([]applicationmodel.ApplicationSnapshot, bool) {
	out := make([]applicationmodel.ApplicationSnapshot, constants.DefaultInitValue)
	for i := range snapshots {
		if snapshots[i].Generation == gen {
			out = append(out, snapshots[i])
		}
	}
	return out, len(out) > constants.DefaultInitValue
}

func newRollbackID() (string, error) {
	u := uuid.New()
	hexStr := hex.EncodeToString(u[:])
	if len(hexStr) < rollbackIDSuffixLen {
		return constants.EmptyString, errors.New("uuid hex too short")
	}
	return fmt.Sprintf("%s%s", rollbackIDPrefix, hexStr[:rollbackIDSuffixLen]), nil
}
