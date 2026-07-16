package notifications

import (
	"encoding/json"
	"fmt"
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
	loggerPref  = "Notifications: "
)

var lg = constants.GetLogger(loggerPref)

func Emit() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := requestutils.ParseRequestBody(r)
		if err != nil {
			respondParseError(w, err)
			return
		}

		var n notiftypes.Notification
		raw, _ := json.Marshal(body)
		if err := json.Unmarshal(raw, &n); err != nil {
			respondParseError(w, err)
			return
		}

		if err := notiftypes.ValidateForEmit(&n); err != nil {
			responseutils.LogAndSendResponse(
				w, http.StatusBadRequest, response.OperationUnprocessed,
				err.Error(), nil, err,
			)
			return
		}
		notiftypes.Truncate(&n)

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		created, err := storage.Emit(r.Context(), n)
		if err != nil {
			lg.Warn(fmt.Sprintf("emit failed: %v", err))
			respondInternal(w, err)
			return
		}

		respondData(w, "notification emitted", structAsMap(*created))
	}
}

func List() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get(queryUserID)
		if userID == constants.EmptyString {
			respondBadRequest(w)
			return
		}

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get(queryLimit))
		cursor := r.URL.Query().Get(queryCursor)

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		resp, err := storage.List(r.Context(), userID, limit, cursor)
		if err != nil {
			lg.Warn(fmt.Sprintf("list failed: %v", err))
			respondInternal(w, err)
			return
		}
		respondData(w, "notifications retrieved", structAsMap(*resp))
	}
}

func MarkRead() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		notificationID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		userID := r.URL.Query().Get(queryUserID)
		if userID == constants.EmptyString {
			respondBadRequest(w)
			return
		}

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		if err := storage.MarkRead(r.Context(), userID, notificationID); err != nil {
			lg.Warn(fmt.Sprintf("markread failed: %v", err))
			respondInternal(w, err)
			return
		}
		respondData(w, "notification marked read", nil)
	}
}

func MarkAllRead() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get(queryUserID)
		if userID == constants.EmptyString {
			respondBadRequest(w)
			return
		}

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		if err := storage.MarkAllRead(r.Context(), userID); err != nil {
			lg.Warn(fmt.Sprintf("markallread failed: %v", err))
			respondInternal(w, err)
			return
		}
		respondData(w, "all notifications marked read", nil)
	}
}

func Clear() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get(queryUserID)
		if userID == constants.EmptyString {
			respondBadRequest(w)
			return
		}

		if !authz.GuardSelfUser(w, r, userID) {
			return
		}

		storage, err := notifstorage.NewStorage()
		if err != nil {
			respondInternal(w, err)
			return
		}
		if err := storage.Clear(r.Context(), userID); err != nil {
			lg.Warn(fmt.Sprintf("clear failed: %v", err))
			respondInternal(w, err)
			return
		}
		respondData(w, "notifications cleared", nil)
	}
}

func respondData(w http.ResponseWriter, msg string, data any) {
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, msg, data, nil)
}

func respondBadRequest(w http.ResponseWriter) {
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationUnprocessed, "userId required", nil, nil)
}

func respondParseError(w http.ResponseWriter, err error) {
	responseutils.LogAndSendResponse(
		w, http.StatusUnprocessableEntity, response.OperationUnprocessed,
		"failed to parse request body", nil, err,
	)
}

func respondInternal(w http.ResponseWriter, err error) {
	responseutils.LogAndSendResponse(
		w, http.StatusInternalServerError, response.OperationError,
		"notifications operation failed", nil, err,
	)
}

func structAsMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
