package notifications

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	notifstorage "github.com/telark/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
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
)

func Emit() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			respondParseError(w, err)
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
