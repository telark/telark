package request

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/rest/base"
	"github.com/telark/rest/utils/request"
)

const validJSONBody = `{"key": "value"}`

func TestParseRequestBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "Valid JSON", body: validJSONBody, wantErr: false},
		{name: "Empty Body", body: "", wantErr: true},
		{name: "Invalid JSON", body: `{"key": "value"`, wantErr: true},
		{name: "Empty Object", body: "{}", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
			if err != nil {
				t.Fatal(err)
			}

			_, err = request.ParseRequestBody(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequestBody() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRequestBodyRejectsOversizeBodies(t *testing.T) {
	const (
		wrapper   = len(`{"k":""}`)
		overLimit = 1
	)
	tests := []struct {
		name     string
		size     int
		tooLarge bool
	}{
		{name: "at the limit", size: int(base.MaxRequestBodySize), tooLarge: false},
		{name: "one byte over", size: int(base.MaxRequestBodySize) + overLimit, tooLarge: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"k":"` + strings.Repeat("a", tt.size-wrapper) + `"}`
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

			_, err := request.ParseRequestBody(req)
			if got := errors.Is(err, request.ErrRequestBodyTooLarge); got != tt.tooLarge {
				t.Fatalf("too large = %v (err %v), want %v", got, err, tt.tooLarge)
			}
			if !tt.tooLarge && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
