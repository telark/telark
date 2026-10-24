package authz

import (
	"net/http"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	maskedUsername = "boot"
	maskedFullname = "Boot Strap"
	maskedEmail    = "test@example.com"
)

func userRecord(id string, bootstrap bool) *unstructured.Unstructured {
	record := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{
		constants.FieldUsername:  maskedUsername,
		constants.FieldFullname:  maskedFullname,
		constants.FieldEmail:     maskedEmail,
		constants.FieldBootstrap: bootstrap,
	}}}
	record.SetName(id)
	return record
}

// Everyone but the bootstrap account itself and the services reads it as "Bootstrap account"; other users stay.
func TestMaskIdentity(t *testing.T) {
	tests := []struct {
		name      string
		identity  *xauthz.Identity
		target    string
		bootstrap bool
		masked    bool
	}{
		{"admin reads the bootstrap account", new(as(userAdmin, allAdmin())), userBootstrap, true, true},
		{"the bootstrap account reads itself", new(as(userBootstrap, allAdmin())), userBootstrap, true, false},
		{"a service reads the bootstrap account", new(internalIdentity), userBootstrap, true, false},
		{"no identity", nil, userBootstrap, true, true},
		{"admin reads a plain user", new(as(userAdmin, allAdmin())), userPlain, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := requestAs(internalIdentity)
			if tt.identity == nil {
				r = r.WithContext(t.Context())
			} else {
				r = requestAs(*tt.identity)
			}
			record := userRecord(tt.target, tt.bootstrap)
			authz.MaskIdentity(r, record)
			want := []string{maskedUsername, maskedFullname, maskedEmail}
			if tt.masked {
				want = []string{constants.BootstrapDisplayName, constants.BootstrapDisplayName, constants.EmptyString}
			}
			for i, field := range []string{constants.FieldUsername, constants.FieldFullname, constants.FieldEmail} {
				if got, _, _ := unstructured.NestedString(record.Object, constants.SpecField, field); got != want[i] {
					t.Fatalf("%s = %q, want %q", field, got, want[i])
				}
			}
		})
	}
}

// A render that masks for its caller is never cached: only the services' and restricted callers' are.
func TestMaskedKey(t *testing.T) {
	key := authz.MaskedKey(func(*http.Request) string { return cacheKey })
	tests := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"internal", requestAs(internalIdentity), cacheKey},
		{"restricted", requestAs(levels(roledata.ScopeUsers, roledata.PermissionLevelOwner)), cacheKey},
		{"admin on all", requestAs(allAdmin()), constants.EmptyString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := key(tt.r); got != tt.want {
				t.Fatalf("key = %q, want %q", got, tt.want)
			}
		})
	}
}

// A record without a spec has nothing to mask and stays as it was.
func TestMaskIdentityLeavesARecordWithoutASpec(t *testing.T) {
	record := &unstructured.Unstructured{Object: map[string]any{}}
	authz.MaskIdentity(requestAs(allAdmin()), record)
	if len(record.Object) != constants.DefaultInitValue {
		t.Fatalf("record = %v, want it untouched", record.Object)
	}
}
