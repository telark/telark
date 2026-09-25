package request

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/telark/rest/utils/request"
)

const (
	actionCreate  = "create"
	validJSONBody = `{"key": "value"}`
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
			body:           validJSONBody,
			action:         actionCreate,
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "Empty Body",
			body:           "",
			action:         actionCreate,
			checkEmptyBody: true,
			wantErr:        true,
		},
		{
			name:           "Invalid JSON",
			body:           `{"key": "value"`,
			action:         actionCreate,
			checkEmptyBody: true,
			wantErr:        true,
		},
		{
			name:           "Get Action",
			body:           validJSONBody,
			action:         "get",
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "List Action",
			body:           validJSONBody,
			action:         "list",
			checkEmptyBody: true,
			wantErr:        false,
		},
		{
			name:           "Empty Body with No Check",
			body:           "{}",
			action:         actionCreate,
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

	_, err = request.ParseRequestBody(req)
	if (err != nil) != tt.wantErr {
		t.Errorf("ParseRequestBody() error = %v, wantErr %v", err, tt.wantErr)
	}
}
