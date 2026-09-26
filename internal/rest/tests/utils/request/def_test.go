package request

import (
	"bytes"
	"net/http"
	"testing"

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
