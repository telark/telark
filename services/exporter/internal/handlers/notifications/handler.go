package notifications

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

const (
	queryUserID = "userId"
	queryLimit  = "limit"
	queryCursor = "cursor"

	msgEmitted       = "notification emitted"
	msgRetrieved     = "notifications retrieved"
	msgMarkedRead    = "notification marked read"
	msgDeleted       = "notification deleted"
	msgAllMarkedRead = "all notifications marked read"
	msgCleared       = "notifications cleared"
	msgUserIDMissing = "userId required"
	msgParseFailed   = "failed to parse request body"
	msgOperationFail = "notifications operation failed"

	msgStreamUserOnly    = "the notification stream needs a user session"
	msgTooManyStreams    = "too many open notification streams"
	msgStreamUnavailable = "notification stream unavailable"

	contentTypeEventStream = "text/event-stream"
	headerCacheControl     = "Cache-Control"
	cacheControlNoCache    = "no-cache, no-transform"
	// nginx and ingress-nginx buffer proxied answers unless told not to, per response.
	headerAccelBuffering = "X-Accel-Buffering"
	accelBufferingOff    = "no"
	frameConnected       = ": connected\n\n"
	frameChanged         = "event: changed\ndata: {}\n\n"
	framePing            = ": ping\n\n"
)

// Says only that the caller's list changed; the client then refetches the page it shows.
// The caller is the session's user, so there is nothing to name or probe.
func Events(hub *notifstorage.Hub, resolver xauthz.Resolver, ping, recheck time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := xauthz.FromContext(r.Context())
		if identity.Internal || identity.UserID == constants.EmptyString {
			responseutils.SendResponse(w, http.StatusForbidden, response.OperationForbidden, msgStreamUserOnly, nil)
			return
		}
		changes, unsubscribe, err := hub.Subscribe(identity.UserID)
		if errors.Is(err, notifstorage.ErrTooManyStreams) {
			responseutils.SendResponse(w, http.StatusTooManyRequests, response.OperationUnavailable, msgTooManyStreams, nil)
			return
		}
		if err != nil {
			responseutils.LogAndSendResponse(w, http.StatusServiceUnavailable, response.OperationUnavailable, msgStreamUnavailable, nil, err)
			return
		}
		defer unsubscribe()
		token := r.Header.Get(dataconstants.HeaderSessionToken)
		stream(w, r, changes, func() bool { return xauthz.SessionEnded(resolver, token) }, ping, recheck)
	}
}

func stream(w http.ResponseWriter, r *http.Request, changes <-chan struct{}, ended func() bool, ping, recheck time.Duration) {
	controller := http.NewResponseController(w)
	// The server's 30 s read and write timeouts would otherwise cut every stream. Only a
	// writer without deadlines refuses, and a dead connection fails the first write anyway.
	_ = controller.SetReadDeadline(time.Time{})
	_ = controller.SetWriteDeadline(time.Time{})
	w.Header().Set(constants.HeaderContentType, contentTypeEventStream)
	w.Header().Set(headerCacheControl, cacheControlNoCache)
	w.Header().Set(headerAccelBuffering, accelBufferingOff)
	write := func(frame string) bool {
		_, err := io.WriteString(w, frame)
		return err == nil && controller.Flush() == nil
	}
	if !write(frameConnected) {
		return
	}
	pings := time.NewTicker(ping)
	defer pings.Stop()
	rechecks := time.NewTicker(recheck)
	defer rechecks.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-changes:
			if !open || !write(frameChanged) {
				return
			}
		case <-pings.C:
			if !write(framePing) {
				return
			}
		case <-rechecks.C:
			if ended() {
				return
			}
		}
	}
}

func Emit() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		n, err := sharedutils.ExtractStructFromBody[notiftypes.Notification](body)
		if err != nil {
			respondParseError(w, err)
			return
		}

		if err := notiftypes.ValidateForEmit(n); err != nil {
			sharedutils.LogByStatusAndSend(
				w, http.StatusBadRequest, response.OperationUnprocessed,
				err.Error(), nil, err,
			)
			return
		}
		notiftypes.Truncate(n)

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		created, err := storage.Emit(r.Context(), *n)
		if err != nil {
			respondInternal(w, err)
			return
		}

		respondData(w, msgEmitted, structAsMap(created))
	}
}

func List() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, storage, ok := userStorage(w, r)
		if !ok {
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get(queryLimit))
		cursor := r.URL.Query().Get(queryCursor)

		resp, err := storage.List(r.Context(), userID, limit, cursor)
		if err != nil {
			respondInternal(w, err)
			return
		}
		respondData(w, msgRetrieved, structAsMap(resp))
	}
}

func MarkRead() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		notificationID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		userID, storage, ok := userStorage(w, r)
		if !ok {
			return
		}

		if err := storage.MarkRead(r.Context(), userID, notificationID); err != nil {
			respondItemError(w, err)
			return
		}
		respondData(w, msgMarkedRead, nil)
	}
}

func Delete() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		notificationID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		userID, storage, ok := userStorage(w, r)
		if !ok {
			return
		}

		if err := storage.Delete(r.Context(), userID, notificationID); err != nil {
			respondItemError(w, err)
			return
		}
		respondData(w, msgDeleted, nil)
	}
}

func MarkAllRead() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, storage, ok := userStorage(w, r)
		if !ok {
			return
		}

		if err := storage.MarkAllRead(r.Context(), userID); err != nil {
			respondInternal(w, err)
			return
		}
		respondData(w, msgAllMarkedRead, nil)
	}
}

func Clear() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, storage, ok := userStorage(w, r)
		if !ok {
			return
		}

		if err := storage.Clear(r.Context(), userID); err != nil {
			respondInternal(w, err)
			return
		}
		respondData(w, msgCleared, nil)
	}
}

// Every route here is scoped to the caller's own notifications, so the guard
// runs before the store is ever touched.
func userStorage(w http.ResponseWriter, r *http.Request) (string, *notifstorage.Storage, bool) {
	userID := r.URL.Query().Get(queryUserID)
	if userID == constants.EmptyString {
		respondBadRequest(w)
		return constants.EmptyString, nil, false
	}

	if !authz.GuardSelfUser(w, r, userID) {
		return constants.EmptyString, nil, false
	}

	storage, err := notifstorage.NewStorage()
	if err != nil {
		respondInternal(w, err)
		return constants.EmptyString, nil, false
	}
	return userID, storage, true
}

func respondData(w http.ResponseWriter, msg string, data any) {
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, msg, data, nil)
}

// An unknown id and someone else's answer alike, so ids can't be probed.
func respondItemError(w http.ResponseWriter, err error) {
	if errors.Is(err, notifstorage.ErrNotificationNotFound) {
		responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound,
			string(constants.ErrNotificationNotFound), nil, nil)
		return
	}
	respondInternal(w, err)
}

func respondBadRequest(w http.ResponseWriter) {
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationUnprocessed, msgUserIDMissing, nil, nil)
}

func respondParseError(w http.ResponseWriter, err error) {
	sharedutils.LogByStatusAndSend(
		w, http.StatusUnprocessableEntity, response.OperationUnprocessed,
		msgParseFailed, nil, err,
	)
}

func respondInternal(w http.ResponseWriter, err error) {
	responseutils.LogAndSendResponse(
		w, http.StatusInternalServerError, response.OperationError,
		msgOperationFail, nil, err,
	)
}

func structAsMap(v any) map[string]any {
	m, _ := sharedutils.StructToSpecMap(v)
	return m
}
