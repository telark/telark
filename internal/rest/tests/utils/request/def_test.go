package request

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/plsyro/rest/utils/request"
)

func TestParseRequestBody(t *testing.T) {
	tests := getTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runTestCase(t, tt)
		})
	}
}

func getTestCases() []struct {
	name           string
	body           string
	action         string
	checkEmptyBody bool
	wantErr        bool
} {
	return []struct {
		name           string
		body           string
		action         string
		checkEmptyBody bool
		wantErr        bool
	}{
		{
			name:           "Valid JSON",
			body:           `{"key": "value"}`,
			action:         "create",
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "Empty Body",
			body:           "",
			action:         "create",
			checkEmptyBody: true,
			wantErr:        true,
		},
		{
			name:           "Invalid JSON",
			body:           `{"key": "value"`,
			action:         "create",
			checkEmptyBody: true,
			wantErr:        true,
		},
		{
			name:           "Get Action",
			body:           `{"key": "value"}`,
			action:         "get",
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "List Action",
			body:           `{"key": "value"}`,
			action:         "list",
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "Empty Body with No Check",
			body:           "{}",
			action:         "create",
			checkEmptyBody: false,
			wantErr:        false,
		},
	}
}

func runTestCase(t *testing.T, tt struct {
	name           string
	body           string
	action         string
	checkEmptyBody bool
	wantErr        bool
},
) {
	req, err := http.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
	if err != nil {
		t.Fatal(err)
	}

	_, err = request.ParseRequestBody(req, tt.action, tt.checkEmptyBody)
	if (err != nil) != tt.wantErr {
		t.Errorf("ParseRequestBody() error = %v, wantErr %v", err, tt.wantErr)
	}
}
