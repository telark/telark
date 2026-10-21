package bootstrap

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

func StartAssertion(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	var body eps.AssertionBody
	if err := shared.DecodeRequestBody(r, &body); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}
	options, err := bootstrap.StartApproval(r, id, body.Action)
	if err != nil {
		sendApprovalError(w, err, http.StatusInternalServerError)
		return
	}
	shared.SendJSONResponse(w, http.StatusOK, StartResponse{Options: options})
}

// The exporter's verdict on the approval or the deny is relayed: its refusals as they are, its failures opaque.
func FinishAssertion(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	bodyBytes, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}
	resp, err := bootstrap.FinishApproval(r, id, bodyBytes)
	if err != nil {
		sendApprovalError(w, err, http.StatusUnauthorized)
		return
	}
	switch {
	case resp.Status == http.StatusOK:
		shared.SendJSONResponse(w, http.StatusOK, resp.Data)
	case resp.Status < http.StatusInternalServerError:
		shared.SendErrorResponse(w, resp.Status, errors.New(resp.Message))
	default:
		shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)), http.StatusInternalServerError, resp.Message)
	}
}

func requestID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, err := shared.GetPathParam(r, constants.IDPathParam)
	if err != nil || id == constants.EmptyString {
		shared.SendErrorResponse(w, http.StatusBadRequest, fmt.Errorf(string(dataerrors.ErrRestRequiredParam), constants.IDPathParam))
		return constants.EmptyString, false
	}
	return id, true
}

// A refused approval is the caller's answer; only a ceremony auth could not run is logged.
func sendApprovalError(w http.ResponseWriter, err error, fallback int) {
	switch {
	case errors.Is(err, bootstrap.ErrAction):
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
	case errors.Is(err, bootstrap.ErrNotOpen), errors.Is(err, bootstrap.ErrNoPasskey):
		shared.SendErrorResponse(w, http.StatusConflict, err)
	case errors.Is(err, shared.ErrBackendUnavailable):
		shared.SendErrorResponse(w, http.StatusServiceUnavailable, err)
	case shared.IsError(err, constants.ErrChallengeNotFound):
		shared.SendErrorResponse(w, http.StatusUnauthorized, err)
	default:
		status := shared.GetStatusCodeForWebAuthnError(err, fallback)
		if status >= http.StatusInternalServerError {
			shared.HandleError(w, errors.New(string(constants.ErrInternalServerError)), status, err.Error())
			return
		}
		shared.SendErrorResponse(w, status, err)
	}
}
