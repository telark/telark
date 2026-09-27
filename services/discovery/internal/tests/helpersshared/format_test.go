package helpersshared

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	partA          = "a"
	appAPI         = "api"
	queryNamespace = "prod"
	partB          = "b"
)

// The concat helpers join with their separator, and the degenerate zero/one
// argument cases short-circuit.
func TestConcatHelpers(t *testing.T) {
	testutil.Equal(t, "dash empty", shared.ConcatWithDash[string](), "")
	testutil.Equal(t, "dash solo", shared.ConcatWithDash("solo"), "solo")
	testutil.Equal(t, "dash pair", shared.ConcatWithDash(partA, partB), "a-b")
	testutil.Equal(t, "dash ints", shared.ConcatWithDash(constants.DefaultAddValue, constants.TwoValue), "1-2")

	if got := shared.ConcatWithColon(partA, partB); !strings.Contains(got, partA) || !strings.Contains(got, partB) {
		t.Fatalf("colon concat = %q", got)
	}
	if got := shared.ConcatWithPath(partA, partB); !strings.Contains(got, partA) || !strings.Contains(got, partB) {
		t.Fatalf("path concat = %q", got)
	}
}

// FormatKey drops the trailing segment when the name is empty.
func TestFormatKey(t *testing.T) {
	full := shared.FormatKey("Deployment", queryNamespace, appAPI)
	if !strings.Contains(full, appAPI) {
		t.Fatalf("full key missing name: %q", full)
	}
	partial := shared.FormatKey("Deployment", queryNamespace, "")
	if strings.Contains(partial, appAPI) {
		t.Fatalf("partial key should not contain a name: %q", partial)
	}
}

// The pools hand back usable, reset objects.
func TestPools(*testing.T) {
	sb := shared.GetStringBuilder()
	sb.WriteString("x")
	shared.PutStringBuilder(sb)

	buf := shared.GetBuffer()
	buf.WriteString("y")
	shared.PutBuffer(buf)
}

// Query-param helpers read present values and flag missing required ones.
func TestQueryParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?ns=prod", nil)
	if v, ok := shared.GetOptionalQueryParam(r, "ns"); !ok || v != queryNamespace {
		t.Fatalf("optional present = %q,%v", v, ok)
	}
	if _, ok := shared.GetOptionalQueryParam(r, "missing"); ok {
		t.Fatal("optional absent reported present")
	}
	if v, err := shared.GetRequiredQueryParam(httptest.NewRecorder(), r, "ns"); err != nil || v != queryNamespace {
		t.Fatalf("required present = %q,%v", v, err)
	}
	if _, err := shared.GetRequiredQueryParam(httptest.NewRecorder(), r, "missing"); err == nil {
		t.Fatal("required absent did not error")
	}
}

// A path param with no router context in the request resolves to an error.
func TestPathParamNoRouter(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	if _, err := shared.GetPathParam(httptest.NewRecorder(), r, "name"); err == nil {
		t.Fatal("expected error resolving a path param with no router")
	}
}
