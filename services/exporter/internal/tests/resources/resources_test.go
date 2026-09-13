package resources

import (
	"net/http/httptest"
	"testing"

	groupdata "github.com/telark/data/resources/group"
	userdata "github.com/telark/data/resources/user"
	grouputil "github.com/telark/exporter/internal/utils/resources/group"
	resshared "github.com/telark/exporter/internal/utils/resources/shared"
	userutil "github.com/telark/exporter/internal/utils/resources/user"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestInitializeIDs(t *testing.T) {
	var nilPtr *[]string
	resshared.InitializeIDs(nilPtr) // must not panic

	var ids []string
	resshared.InitializeIDs(&ids)
	if ids == nil {
		t.Error("nil slice not initialized to empty")
	}

	existing := []string{"a"}
	resshared.InitializeIDs(&existing)
	if len(existing) != 1 {
		t.Error("existing slice mutated")
	}
}

func TestReplaceIDsIfProvided(t *testing.T) {
	resshared.ReplaceIDsIfProvided[string](nil, "k", nil, nil) // nil body must not panic

	body := map[string]any{"ids": []any{"x"}}
	var target []string
	resshared.ReplaceIDsIfProvided(body, "ids", []string{"a", "b"}, &target)
	if len(target) != 2 {
		t.Errorf("target = %v, want 2 ids", target)
	}

	// Provided-but-nil normalizes to an empty (non-nil) slice.
	body2 := map[string]any{"ids": nil}
	var target2 []string
	resshared.ReplaceIDsIfProvided(body2, "ids", nil, &target2)
	if target2 == nil {
		t.Error("provided nil ids should normalize to empty slice")
	}

	// Key absent from body leaves the target untouched.
	var target3 []string
	resshared.ReplaceIDsIfProvided(map[string]any{}, "ids", []string{"a"}, &target3)
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
	valid := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"a": 1}}}
	rec := httptest.NewRecorder()
	resshared.SendFilteredResourceResponse(rec, valid)
	if rec.Code != 200 {
		t.Errorf("valid resource status = %d, want 200", rec.Code)
	}

	invalid := &unstructured.Unstructured{Object: map[string]any{}}
	rec2 := httptest.NewRecorder()
	resshared.SendFilteredResourceResponse(rec2, invalid)
	if rec2.Code != 500 {
		t.Errorf("invalid resource status = %d, want 500", rec2.Code)
	}
}

func TestSendFilteredResourcesResponse(t *testing.T) {
	valid := []*unstructured.Unstructured{
		{Object: map[string]any{"spec": map[string]any{"a": 1}}},
	}
	rec := httptest.NewRecorder()
	resshared.SendFilteredResourcesResponse(rec, valid)
	if rec.Code != 200 {
		t.Errorf("valid list status = %d, want 200", rec.Code)
	}

	withBad := []*unstructured.Unstructured{{Object: map[string]any{}}}
	rec2 := httptest.NewRecorder()
	resshared.SendFilteredResourcesResponse(rec2, withBad)
	if rec2.Code != 500 {
		t.Errorf("list with unfilterable item status = %d, want 500", rec2.Code)
	}
}

func TestExtractUserFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"username": "u1"}}}
	user, err := userutil.ExtractUserFromUnstructured(res)
	if err != nil || user == nil || user.Username != "u1" {
		t.Fatalf("ExtractUserFromUnstructured = %+v, err %v", user, err)
	}
	empty, err := userutil.ExtractUserFromUnstructured(&unstructured.Unstructured{Object: map[string]any{}})
	if err != nil || empty != nil {
		t.Errorf("missing spec should be nil user, got %+v err %v", empty, err)
	}
}

func TestExtractUserSpecDefaults(t *testing.T) {
	user, err := userutil.ExtractUserSpecFromRequestBody(map[string]any{"username": "u"})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if user.Status.Phase != string(userdata.AccountPhaseActive) {
		t.Errorf("phase default = %q, want active", user.Status.Phase)
	}
	if user.AssignedRolesIDs == nil || user.AssignedGroupsIDs == nil {
		t.Error("assigned id slices not initialized")
	}
}

func TestMergeUserAndPreparePatchBody(t *testing.T) {
	existing := &userdata.UserAsResource{Username: "old", Fullname: "old-full", Email: "old@e.io"}
	next := &userdata.UserAsResource{Username: "new"}
	body := map[string]any{"username": "new"}
	merged := userutil.MergeUserAndPreparePatchBody(existing, next, body)
	if merged.Username != "new" {
		t.Errorf("username not merged: %q", merged.Username)
	}
	// Fullname was not part of the patch body, so it must be preserved.
	if merged.Fullname != "old-full" {
		t.Errorf("fullname clobbered: %q", merged.Fullname)
	}
}

func TestMergeUserSettingsIntoPatchBody(t *testing.T) {
	existing := &userdata.UserAsResource{Username: "u", Avatar: &userdata.Avatar{Style: "s", Seed: "x"}}
	next := &userdata.UserAsResource{Settings: &userdata.UserSettings{Timezone: "Europe/Paris", Region: "FR"}}
	body := map[string]any{"settings": map[string]any{"timezone": "Europe/Paris", "region": "FR"}}
	merged := userutil.MergeUserAndPreparePatchBody(existing, next, body)
	if merged.Settings == nil || merged.Settings.Timezone != "Europe/Paris" || merged.Settings.Region != "FR" {
		t.Fatalf("settings not merged: %+v", merged.Settings)
	}
	if body["settings"] != next.Settings {
		t.Errorf("patch body settings not normalized: %#v", body["settings"])
	}
	if merged.Avatar == nil || merged.Avatar.Seed != "x" {
		t.Errorf("avatar clobbered: %+v", merged.Avatar)
	}
}

func TestExtractAndMergeUserForPatch(t *testing.T) {
	existing := &userdata.UserAsResource{Username: "old"}
	body := map[string]any{"fullname": "New Name"}
	rec := httptest.NewRecorder()
	if !userutil.ExtractAndMergeUserForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeUserForPatch returned false")
	}
}

// An unchanged username must never trigger the cluster uniqueness lookup.
func TestExtractAndMergeUserForPatchSkipsUsernameCheckWhenUnchanged(t *testing.T) {
	existing := &userdata.UserAsResource{Username: "old"}
	body := map[string]any{"username": "old"}
	rec := httptest.NewRecorder()
	if !userutil.ExtractAndMergeUserForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeUserForPatch returned false for an unchanged username")
	}
}

func TestUserValidationEmptyUsername(t *testing.T) {
	if err := userutil.CheckUsernameExists(""); err == nil {
		t.Error("empty username accepted")
	}
	// No identities + empty username fails before any cluster call.
	rec := httptest.NewRecorder()
	if err := userutil.ValidateAndPrepareUser(&userdata.UserAsResource{}, rec); err == nil {
		t.Error("empty username user accepted")
	}
}

func TestExtractGroupFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"name": "g1"}}}
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
	group, err := grouputil.ExtractGroupSpecFromRequestBody(map[string]any{"name": "g"})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if group.AssignedUsersIDs == nil || group.AssignedRolesIDs == nil {
		t.Error("group id slices not initialized")
	}
}

func TestMergeGroupAndPreparePatchBody(t *testing.T) {
	created := "creator"
	existing := &groupdata.GroupAsResource{Name: "old", Description: "old-d", CategoryID: "c"}
	next := &groupdata.GroupAsResource{Name: "new", CreatedBy: &created}
	body := map[string]any{}
	merged := grouputil.MergeGroupAndPreparePatchBody(existing, next, body)
	if merged.Name != "new" || merged.Description != "old-d" {
		t.Errorf("merge wrong: %+v", merged)
	}
	if body["createdBy"] != "creator" {
		t.Errorf("createdBy not written to body: %v", body["createdBy"])
	}
}

func TestExtractAndMergeGroupForPatch(t *testing.T) {
	existing := &groupdata.GroupAsResource{Name: "old"}
	body := map[string]any{"name": "renamed", "assignedUsersIDs": []any{"u1"}}
	rec := httptest.NewRecorder()
	if !grouputil.ExtractAndMergeGroupForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeGroupForPatch returned false")
	}
}

func TestGroupValidationEmptyName(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := grouputil.ValidateAndPrepareGroup(&groupdata.GroupAsResource{}, rec); err == nil {
		t.Error("empty group name accepted")
	}
}
