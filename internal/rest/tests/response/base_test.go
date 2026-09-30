package response

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/telark/internal/rest/response"
)

const (
	singleWrite = 1
	jsonType    = "application/json"
	noBody      = ""
	encodedOK   = `{"status":200,"operation":"Success","message":"ok"}` + "\n"
	encodedList = `[{"status":201,"operation":"Success","message":"ok"}]` + "\n"
)

type countingRecorder struct {
	*httptest.ResponseRecorder
	headerWrites int
}

func (r *countingRecorder) WriteHeader(code int) {
	r.headerWrites++
	r.ResponseRecorder.WriteHeader(code)
}

func newRecorder() *countingRecorder {
	return &countingRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func assertWritten(t *testing.T, rec *countingRecorder, status int, contentType, body string) {
	t.Helper()
	if rec.headerWrites != singleWrite {
		t.Errorf("WriteHeader called %d times, want exactly once", rec.headerWrites)
	}
	if rec.Code != status {
		t.Errorf("status = %d, want %d", rec.Code, status)
	}
	if got := rec.Header().Get("Content-Type"); got != contentType {
		t.Errorf("Content-Type = %q, want %q", got, contentType)
	}
	if got := rec.Body.String(); got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
}

func okResponse(status int) *response.GenericResponse {
	return response.NewGenericResponse(status, response.OperationSuccess, nil, "ok")
}

func TestEncodeJSONResponseWritesHeaderOnce(t *testing.T) {
	t.Run("non-empty keeps status and body", func(t *testing.T) {
		rec := newRecorder()
		response.EncodeJSONResponse(rec, http.StatusOK, okResponse(http.StatusOK))
		assertWritten(t, rec, http.StatusOK, jsonType, encodedOK)
	})

	t.Run("nil is no content", func(t *testing.T) {
		rec := newRecorder()
		response.EncodeJSONResponse(rec, http.StatusOK, nil)
		assertWritten(t, rec, http.StatusNoContent, noBody, noBody)
	})

	t.Run("unencodable payload is a 500", func(t *testing.T) {
		rec := newRecorder()
		bad := response.NewGenericResponse(http.StatusOK, response.OperationSuccess, make(chan int), noBody)
		response.EncodeJSONResponse(rec, http.StatusOK, bad)
		if rec.headerWrites != singleWrite || rec.Code != http.StatusInternalServerError {
			t.Errorf("got %d writes with status %d, want one 500", rec.headerWrites, rec.Code)
		}
	})
}

func TestEncodeMultiJSONResponseWritesHeaderOnce(t *testing.T) {
	t.Run("non-empty keeps status and body", func(t *testing.T) {
		rec := newRecorder()
		response.EncodeMultiJSONResponse(rec, http.StatusCreated,
			[]*response.GenericResponse{okResponse(http.StatusCreated)})
		assertWritten(t, rec, http.StatusCreated, jsonType, encodedList)
	})

	for name, responses := range map[string][]*response.GenericResponse{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name+" is no content", func(t *testing.T) {
			rec := newRecorder()
			response.EncodeMultiJSONResponse(rec, http.StatusOK, responses)
			assertWritten(t, rec, http.StatusNoContent, noBody, noBody)
		})
	}
}
