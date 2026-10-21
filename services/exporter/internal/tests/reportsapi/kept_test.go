package reportsapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	plansmd "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/exporter/internal/constants"
	protectionhandler "github.com/telark/telark/services/exporter/internal/handlers/plans/protection"
	reportshandler "github.com/telark/telark/services/exporter/internal/handlers/reports"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	keptName      = "Payments freeze"
	keepTrue      = "true"
	keepInvalid   = "maybe"
	listSuffix    = "List"
	keyAPIVersion = "apiVersion"
	keyKind       = "kind"
	keyMetadata   = "metadata"
	keyNamespace  = "namespace"
	keptPath      = "/reports/kept"
)

var (
	planMD  = plansmd.ProtectionPlanMetadata
	planGVR = schema.GroupVersionResource{Group: planMD.BaseGroup, Version: planMD.Version, Resource: planMD.Plural}
)

// The fake would guess the plural from the kind, so the plan goes in through the resource client.
func installPlan(t *testing.T) *dynamicfake.FakeDynamicClient {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{planGVR: planMD.Kind + listSuffix})
	plan := &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion:       planMD.GetAPIVersion(),
		keyKind:             planMD.Kind,
		keyMetadata:         map[string]any{constants.FieldName: planID, keyNamespace: planMD.Namespace},
		constants.SpecField: map[string]any{constants.FieldName: keptName},
	}}
	if _, err := client.Resource(planGVR).Namespace(planMD.Namespace).Create(context.Background(), plan, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	return client
}

func deletePlan(query url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	protectionhandler.DeletePlanByID()(rec, withID(httptest.NewRequest(http.MethodDelete, routePath+"?"+query.Encode(), nil)))
	return rec
}

func deleteReport(report string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	query := url.Values{reportseps.QueryReport: {report}}
	reportshandler.DeletePlanReport()(rec, withID(httptest.NewRequest(http.MethodDelete, routePath+"?"+query.Encode(), nil)))
	return rec
}

type deleteStep struct {
	name   string
	report string
	want   int
}

var deleteSteps = []deleteStep{
	{"no report", constants.EmptyString, http.StatusBadRequest},
	{"the report", reportID, http.StatusOK},
	{"already deleted", reportID, http.StatusNotFound},
}

func runDeleteSteps(t *testing.T) {
	t.Helper()
	for _, step := range deleteSteps {
		if rec := deleteReport(step.report); rec.Code != step.want {
			t.Fatalf(statusFmt, step.name, rec.Code, step.want)
		}
	}
	if rec := download(reportID, reportseps.FormatHTML); rec.Code != http.StatusNotFound {
		t.Fatalf(statusFmt, "download after delete", rec.Code, http.StatusNotFound)
	}
}

func TestDeleteRemovesALiveReport(t *testing.T) {
	setRoot(t)
	create(t, createBody(t, actor))
	runDeleteSteps(t)

	list := httptest.NewRecorder()
	reportshandler.ListPlanReports()(list, withID(httptest.NewRequest(http.MethodGet, routePath, nil)))
	if metas := decodeData[[]reportseps.ReportMeta](t, list); len(metas) != constants.DefaultInitValue {
		t.Fatalf("list after delete = %+v, want none", metas)
	}
}

func TestDeletePlanKeepsItsReportsOnlyWhenAsked(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query url.Values
		kept  bool
	}{
		{"absent removes them", url.Values{}, false},
		{"true keeps them", url.Values{planseps.QueryKeepReports: {keepTrue}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := setRoot(t)
			installPlan(t)
			create(t, createBody(t, actor))
			if rec := deletePlan(tc.query); rec.Code != http.StatusOK {
				t.Fatalf(statusFmt, tc.name, rec.Code, http.StatusOK)
			}
			if exists(reportsutil.PlanDir(root, planID)) {
				t.Fatal("the reports are still under plans after the delete")
			}
			if got := exists(reportsutil.KeptDir(root, planID)); got != tc.kept {
				t.Fatalf("kept directory present = %v, want %v", got, tc.kept)
			}
		})
	}
}

func TestDeletePlanRefusesAnInvalidKeepReports(t *testing.T) {
	root := setRoot(t)
	client := installPlan(t)
	create(t, createBody(t, actor))
	if rec := deletePlan(url.Values{planseps.QueryKeepReports: {keepInvalid}}); rec.Code != http.StatusBadRequest {
		t.Fatalf(statusFmt, keepInvalid, rec.Code, http.StatusBadRequest)
	}
	if _, err := client.Resource(planGVR).Namespace(planMD.Namespace).Get(context.Background(), planID, metav1.GetOptions{}); err != nil {
		t.Fatalf("a refused delete removed the plan: %v", err)
	}
	if !exists(reportsutil.PlanDir(root, planID)) {
		t.Fatal("a refused delete removed the reports")
	}
}

func TestKeptReportsListDownloadAndDelete(t *testing.T) {
	root := setRoot(t)
	create(t, createBody(t, actor))
	if err := reportsutil.Keep(root, planID, keptName); err != nil {
		t.Fatal(err)
	}

	list := httptest.NewRecorder()
	reportshandler.ListKeptReports()(list, httptest.NewRequest(http.MethodGet, keptPath, nil))
	kept := decodeData[[]reportseps.KeptPlanReports](t, list)
	if list.Code != http.StatusOK || len(kept) != constants.DefaultIncrementValue || kept[firstIdx].PlanName != keptName ||
		len(kept[firstIdx].Reports) != constants.DefaultIncrementValue || kept[firstIdx].Reports[firstIdx].ID != reportID {
		t.Fatalf("kept list = %d %+v, want plan %s with report %s", list.Code, kept, keptName, reportID)
	}
	if rec := download(reportID, reportseps.FormatHTML); rec.Code != http.StatusOK || rec.Body.String() != actor+"-"+reportseps.FormatHTML {
		t.Fatalf("kept download = %d %q", rec.Code, rec.Body.String())
	}

	runDeleteSteps(t)
	if exists(reportsutil.KeptDir(root, planID)) {
		t.Fatal("the kept directory outlived its last report")
	}
}
