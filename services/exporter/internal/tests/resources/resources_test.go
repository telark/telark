package resources

import (
	"net/http"
	"net/http/httptest"
	"testing"

	groupdata "github.com/telark/data/resources/group"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	grouputil "github.com/telark/exporter/internal/utils/resources/group"
	resshared "github.com/telark/exporter/internal/utils/resources/shared"
	userutil "github.com/telark/exporter/internal/utils/resources/user"
	xauthz "github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	valueA     = "a"
	valueX     = "x"
	nameOld    = "old"
	nameNew    = "new"
	testUserID = "u1"

	idsKey          = "ids"
	settingsKey     = "settings"
	testTimezone    = "Europe/Paris"
	testRegion      = "FR"
	wantReplacedIDs = 2
)

func TestInitializeIDs(t *testing.T) {
	var nilPtr *[]string
	resshared.InitializeIDs(nilPtr) // must not panic

	var ids []string
	resshared.InitializeIDs(&ids)
	if ids == nil {
		t.Error("nil slice not initialized to empty")
	}

	existing := []string{valueA}
	resshared.InitializeIDs(&existing)
	if len(existing) != constants.DefaultIncrementValue {
		t.Error("existing slice mutated")
	}
}

func TestReplaceIDsIfProvided(t *testing.T) {
	resshared.ReplaceIDsIfProvided[string](nil, "k", nil, nil) // nil body must not panic

	body := map[string]any{idsKey: []any{valueX}}
	var target []string
	resshared.ReplaceIDsIfProvided(body, idsKey, []string{valueA, "b"}, &target)
	if len(target) != wantReplacedIDs {
		t.Errorf("target = %v, want 2 ids", target)
	}

	// Provided-but-nil normalizes to an empty (non-nil) slice.
	body2 := map[string]any{idsKey: nil}
	var target2 []string
	resshared.ReplaceIDsIfProvided(body2, idsKey, nil, &target2)
	if target2 == nil {
		t.Error("provided nil ids should normalize to empty slice")
	}

	// Key absent from body leaves the target untouched.
	var target3 []string
	resshared.ReplaceIDsIfProvided(map[string]any{}, idsKey, []string{valueA}, &target3)
	if target3 != nil {
		t.Error("absent key should not replace target")
	}
}

func TestAddLastUpdateDateToPatchBody(t *testing.T) {
	body := map[string]any{}
	resshared.AddLastUpdateDateToPatchBody(body)
	if _, ok := body["lastUpdateDate"]; !ok {
		t.Error("lastUpdateDate not added")
	}
	resshared.AddLastUpdateDateToPatchBody(nil) // must not panic
}

func TestSendFilteredResourceResponse(t *testing.T) {
	valid := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{valueA: constants.DefaultIncrementValue}}}
	rec := httptest.NewRecorder()
	resshared.SendFilteredResourceResponse(rec, valid)
	if rec.Code != http.StatusOK {
		t.Errorf("valid resource status = %d, want 200", rec.Code)
	}

	invalid := &unstructured.Unstructured{Object: map[string]any{}}
	rec2 := httptest.NewRecorder()
	resshared.SendFilteredResourceResponse(rec2, invalid)
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("invalid resource status = %d, want 500", rec2.Code)
	}
}

func TestSendFilteredResourcesResponse(t *testing.T) {
	valid := []*unstructured.Unstructured{
		{Object: map[string]any{constants.SpecField: map[string]any{valueA: constants.DefaultIncrementValue}}},
	}
	rec := httptest.NewRecorder()
	resshared.SendFilteredResourcesResponse(rec, valid)
	if rec.Code != http.StatusOK {
		t.Errorf("valid list status = %d, want 200", rec.Code)
	}

	withBad := []*unstructured.Unstructured{{Object: map[string]any{}}}
	rec2 := httptest.NewRecorder()
	resshared.SendFilteredResourcesResponse(rec2, withBad)
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("list with unfilterable item status = %d, want 500", rec2.Code)
	}
}

func TestExtractUserFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{constants.UsernameParam: testUserID}}}
	user, err := userutil.ExtractUserFromUnstructured(res)
	if err != nil || user == nil || user.Username != testUserID {
		t.Fatalf("ExtractUserFromUnstructured = %+v, err %v", user, err)
	}
	empty, err := userutil.ExtractUserFromUnstructured(&unstructured.Unstructured{Object: map[string]any{}})
	if err != nil || empty != nil {
		t.Errorf("missing spec should be nil user, got %+v err %v", empty, err)
	}
}

func TestExtractUserSpecDefaults(t *testing.T) {
	user, err := userutil.ExtractUserSpecFromRequestBody(map[string]any{constants.UsernameParam: "u"})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if user.Status.Phase != string(userdata.AccountPhaseActive) {
		t.Errorf("phase default = %q, want active", user.Status.Phase)
	}
	if user.RoleRefs == nil || user.GroupRefs == nil {
		t.Error("assigned id slices not initialized")
	}
}

func TestMergeUserAndPreparePatchBody(t *testing.T) {
	existing := &userdata.User{Username: nameOld, Fullname: "old-full", Email: "old@e.io"}
	next := &userdata.User{Username: nameNew}
	body := map[string]any{constants.UsernameParam: nameNew}
	merged := userutil.MergeUserAndPreparePatchBody(existing, next, body)
	if merged.Username != nameNew {
		t.Errorf("username not merged: %q", merged.Username)
	}
	// Fullname was not part of the patch body, so it must be preserved.
	if merged.Fullname != "old-full" {
		t.Errorf("fullname clobbered: %q", merged.Fullname)
	}
}

func TestMergeUserSettingsIntoPatchBody(t *testing.T) {
	existing := &userdata.User{Username: "u", Avatar: &userdata.Avatar{Style: "s", Seed: valueX}}
	next := &userdata.User{Settings: &userdata.UserSettings{Timezone: testTimezone, Region: testRegion}}
	body := map[string]any{settingsKey: map[string]any{"timezone": testTimezone, "region": testRegion}}
	merged := userutil.MergeUserAndPreparePatchBody(existing, next, body)
	if merged.Settings == nil || merged.Settings.Timezone != testTimezone || merged.Settings.Region != testRegion {
		t.Fatalf("settings not merged: %+v", merged.Settings)
	}
	if body[settingsKey] != next.Settings {
		t.Errorf("patch body settings not normalized: %#v", body[settingsKey])
	}
	if merged.Avatar == nil || merged.Avatar.Seed != valueX {
		t.Errorf("avatar clobbered: %+v", merged.Avatar)
	}
}

func TestExtractAndMergeUserForPatch(t *testing.T) {
	existing := &userdata.User{Username: nameOld}
	body := map[string]any{"fullname": "New Name"}
	rec := httptest.NewRecorder()
	if !userutil.ExtractAndMergeUserForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeUserForPatch returned false")
	}
}

// An unchanged username must never trigger the cluster uniqueness lookup.
func TestExtractAndMergeUserForPatchSkipsUsernameCheckWhenUnchanged(t *testing.T) {
	existing := &userdata.User{Username: nameOld}
	body := map[string]any{constants.UsernameParam: nameOld}
	rec := httptest.NewRecorder()
	if !userutil.ExtractAndMergeUserForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeUserForPatch returned false for an unchanged username")
	}
}

func TestUserValidationEmptyUsername(t *testing.T) {
	if err := userutil.CheckUsernameExists(constants.EmptyString); err == nil {
		t.Error("empty username accepted")
	}
	// No identities + empty username fails before any cluster call.
	rec := httptest.NewRecorder()
	if err := userutil.ValidateAndPrepareUser(&userdata.User{}, rec); err == nil {
		t.Error("empty username user accepted")
	}
}

func TestExtractGroupFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{constants.NameParam: "g1"}}}
	group, err := grouputil.ExtractGroupFromUnstructured(res)
	if err != nil || group == nil || group.Name != "g1" {
		t.Fatalf("ExtractGroupFromUnstructured = %+v, err %v", group, err)
	}
	empty, err := grouputil.ExtractGroupFromUnstructured(&unstructured.Unstructured{Object: map[string]any{}})
	if err != nil || empty != nil {
		t.Errorf("missing spec should be nil group, got %+v", empty)
	}
}

func TestExtractGroupSpecInitializesIDs(t *testing.T) {
	group, err := grouputil.ExtractGroupSpecFromRequestBody(map[string]any{constants.NameParam: "g"})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if group.UserRefs == nil || group.RoleRefs == nil {
		t.Error("group id slices not initialized")
	}
}

func TestMergeGroupAndPreparePatchBody(t *testing.T) {
	created := "creator"
	existing := &groupdata.Group{Name: nameOld, Description: "old-d", CategoryRef: "c"}
	next := &groupdata.Group{Name: nameNew, CreatedBy: &created}
	body := map[string]any{}
	merged := grouputil.MergeGroupAndPreparePatchBody(existing, next, body)
	if merged.Name != nameNew || merged.Description != "old-d" {
		t.Errorf("merge wrong: %+v", merged)
	}
	if forged, written := body["createdBy"]; written {
		t.Errorf("a body createdBy must not reach the patch: %v", forged)
	}
}

func TestExtractAndMergeGroupForPatch(t *testing.T) {
	existing := &groupdata.Group{Name: nameOld}
	body := map[string]any{constants.NameParam: "renamed", "userRefs": []any{testUserID}}
	rec := httptest.NewRecorder()
	if !grouputil.ExtractAndMergeGroupForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeGroupForPatch returned false")
	}
}

func TestGroupValidationEmptyName(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := grouputil.ValidateAndPrepareGroup(&groupdata.Group{}, rec); err == nil {
		t.Error("empty group name accepted")
	}
}

// Audit actors are the authenticated caller: a body value never survives, and a
// service call naming no user leaves the stored actors alone.
func TestStampAudit(t *testing.T) {
	caller := xauthz.Identity{UserID: testUserID}
	tests := []struct {
		name                     string
		identity                 xauthz.Identity
		stamp                    func(*http.Request, map[string]any)
		wantCreated, wantUpdated any
	}{
		{"create", caller, resshared.StampCreateAudit, testUserID, testUserID},
		{"patch", caller, resshared.StampPatchAudit, nil, testUserID},
		{"service call naming no user", xauthz.Identity{Internal: true}, resshared.StampCreateAudit, nil, nil},
		{"service call forwarding its caller", xauthz.Identity{Internal: true, UserID: testUserID}, resshared.StampPatchAudit, nil, testUserID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
			r = r.WithContext(xauthz.WithIdentity(r.Context(), tt.identity))
			body := map[string]any{constants.FieldCreatedBy: valueX, constants.FieldLastUpdatedBy: valueX}
			tt.stamp(r, body)
			created, updated := body[constants.FieldCreatedBy], body[constants.FieldLastUpdatedBy]
			if created != tt.wantCreated || updated != tt.wantUpdated {
				t.Errorf("audit = %v / %v, want %v / %v", created, updated, tt.wantCreated, tt.wantUpdated)
			}
		})
	}
}
