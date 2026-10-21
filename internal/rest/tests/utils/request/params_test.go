package request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/telark/internal/rest/utils/request"
)

const boolParam = "keep"

func TestOptionalBoolQuery(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    bool
		wantErr bool
	}{
		{name: "absent reads as false", target: "/"},
		{name: "true", target: "/?keep=true", want: true},
		{name: "false", target: "/?keep=false"},
		{name: "other ParseBool spellings", target: "/?keep=1", want: true},
		{name: "a typo is refused", target: "/?keep=yes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := request.OptionalBoolQuery(httptest.NewRequest(http.MethodDelete, tt.target, nil), boolParam)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("OptionalBoolQuery(%s) = (%v, %v), want (%v, error %v)", tt.target, got, err, tt.want, tt.wantErr)
			}
		})
	}
}
