package migrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/migrations"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

const (
	verbGet         = "get"
	verbPatch       = "patch"
	subStatus       = "status"
	listSuffix      = "List"
	stepName        = "test step"
	crdFile         = "../../../../../charts/telark-crds/templates/crds/telarkconfigs.yaml"
	templateOpen    = "{{"
	lineBreak       = "\n"
	typeInteger     = "integer"
	oldestRelease   = "1.0.0"
	previousRelease = "1.1.0"
	newestRelease   = "1.2.0"
)

const (
	noRecord        int64 = -1
	noSteps         int64 = 0
	noFailure       int64 = 0
	noPreviousSteps int64 = 0
	firstStep       int64 = 1
	secondStep      int64 = 2
	lastStep        int64 = 3
	fourthStep      int64 = 4
	newerStep       int64 = 5
)

type migrationRecord struct {
	step, rollback int64
}

var (
	md        = v1alpha1.TelarkConfigMetadata
	configGVR = schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
	errStep   = errors.New("step failed")
	none      = migrationRecord{noRecord, noRecord}
)

func config(record migrationRecord) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": md.GetAPIVersion(),
		"kind":       md.Kind,
		"metadata":   map[string]any{"name": constants.TelarkConfigResourceName, "namespace": md.Namespace},
		"spec":       map[string]any{},
	}}
	status := map[string]any{}
	if record.step != noRecord {
		status[constants.FieldLastMigration] = record.step
	}
	if record.rollback != noRecord {
		status[constants.FieldRollbackMigration] = record.rollback
	}
	if len(status) > constants.DefaultInitValue {
		obj.Object[constants.FieldStatus] = status
	}
	return obj
}

// The fake would guess the plural from the kind, so objects go in through the resource client.
func installFake(t *testing.T, objects ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{configGVR: md.Kind + listSuffix})
	for _, obj := range objects {
		if _, err := client.Resource(configGVR).Namespace(md.Namespace).Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	client.ClearActions()
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	return client
}

// Steps 1..count of one release that log their number to ran; the one numbered failAt fails.
func numberedSteps(count, failAt int64, ran *[]int64) []migrations.Step {
	steps := make([]migrations.Step, constants.DefaultInitValue, count)
	for number := int64(constants.DefaultIncrementValue); number <= count; number++ {
		steps = append(steps, migrations.Step{Number: number, Release: newestRelease, Name: stepName, Apply: func() error {
			*ran = append(*ran, number)
			if number == failAt {
				return errStep
			}
			return nil
		}})
	}
	return steps
}

func releaseSteps(ran *[]int64, releases ...string) []migrations.Step {
	steps := numberedSteps(int64(len(releases)), noFailure, ran)
	for i, release := range releases {
		steps[i].Release = release
	}
	return steps
}

func readRecord(t *testing.T, obj map[string]any) migrationRecord {
	t.Helper()
	return migrationRecord{readField(t, obj, constants.FieldLastMigration), readField(t, obj, constants.FieldRollbackMigration)}
}

func readField(t *testing.T, obj map[string]any, field string) int64 {
	t.Helper()
	value, found, err := unstructured.NestedInt64(obj, constants.FieldStatus, field)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return noRecord
	}
	return value
}

func storedRecord(t *testing.T, client *dynamicfake.FakeDynamicClient) migrationRecord {
	t.Helper()
	obj, err := client.Resource(configGVR).Namespace(md.Namespace).Get(context.Background(), constants.TelarkConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return readRecord(t, obj.Object)
}

// The record each write carried, in order; every write must patch TelarkConfig's status subresource.
func recordedSteps(t *testing.T, client *dynamicfake.FakeDynamicClient) []migrationRecord {
	t.Helper()
	var written []migrationRecord
	for _, action := range client.Actions() {
		if action.GetVerb() == verbGet {
			continue
		}
		patch, isPatch := action.(k8stesting.PatchAction)
		if !isPatch || patch.GetResource().Resource != md.Plural || patch.GetSubresource() != subStatus {
			t.Fatalf("unexpected write: %s %s/%s", action.GetVerb(), action.GetResource().Resource, action.GetSubresource())
		}
		var body map[string]any
		if err := utiljson.Unmarshal(patch.GetPatch(), &body); err != nil {
			t.Fatal(err)
		}
		written = append(written, readRecord(t, body))
	}
	return written
}

func records(rollback int64, steps ...int64) []migrationRecord {
	out := make([]migrationRecord, constants.DefaultInitValue, len(steps))
	for _, step := range steps {
		out = append(out, migrationRecord{step, rollback})
	}
	return out
}

// Each step is recorded as it completes, so the next start resumes after the last recorded one.
// A record one release newer than every known step is lowered to the last known step, and a record
// further ahead refuses the start before anything runs or is written.
func TestRunAppliesPendingStepsAndRecordsEach(t *testing.T) {
	all := []int64{firstStep, secondStep, lastStep}
	migrated := migrationRecord{lastStep, noPreviousSteps}
	tests := []struct {
		name         string
		stored       migrationRecord
		steps        int64
		failAt       int64
		wantRan      []int64
		wantRecorded []int64
		want         migrationRecord
		wantErr      bool
	}{
		{"fresh install runs every step", none, lastStep, noFailure, all, all, migrated, false},
		{"resumes after the recorded step", migrationRecord{firstStep, noPreviousSteps}, lastStep, noFailure,
			all[firstStep:], all[firstStep:], migrated, false},
		{"a failed step keeps the record before it", none, lastStep, secondStep,
			all[:secondStep], all[:firstStep], migrationRecord{firstStep, noPreviousSteps}, true},
		{"nothing pending", migrated, lastStep, noFailure, nil, nil, migrated, false},
		{"no steps and no record", none, noSteps, noFailure, nil, nil, none, false},
		{"one release back lowers the record and keeps its rollback step", migrationRecord{newerStep, lastStep},
			lastStep, noFailure, nil, []int64{lastStep}, migrationRecord{lastStep, lastStep}, false},
		{"one release back without steps lowers the record to 0", migrationRecord{secondStep, noPreviousSteps},
			noSteps, noFailure, nil, []int64{noSteps}, migrationRecord{noSteps, noPreviousSteps}, false},
		{"refuses data more than one release newer", migrationRecord{newerStep, fourthStep},
			lastStep, noFailure, nil, nil, migrationRecord{newerStep, fourthStep}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, config(tt.stored))
			var ran []int64

			err := migrations.Run(numberedSteps(tt.steps, tt.failAt, &ran))

			if (err != nil) != tt.wantErr {
				t.Fatalf("Run error = %v, want error %v", err, tt.wantErr)
			}
			if tt.failAt != noFailure && !errors.Is(err, errStep) {
				t.Errorf("Run error = %v, want it to wrap the step's error", err)
			}
			if !slices.Equal(ran, tt.wantRan) {
				t.Errorf("steps run = %v, want %v", ran, tt.wantRan)
			}
			if got, want := recordedSteps(t, client), records(tt.want.rollback, tt.wantRecorded...); !slices.Equal(got, want) {
				t.Errorf("records written = %v, want %v", got, want)
			}
			if got := storedRecord(t, client); got != tt.want {
				t.Errorf("stored record = %v, want %v", got, tt.want)
			}
		})
	}
}

// Every write records the last step of the release before the newest step's release.
func TestRunRecordsTheLastStepOfThePreviousRelease(t *testing.T) {
	tests := []struct {
		name     string
		releases []string
		want     int64
	}{
		{"one release", []string{newestRelease, newestRelease, newestRelease}, noPreviousSteps},
		{"two releases", []string{previousRelease, newestRelease, newestRelease}, firstStep},
		{"three releases", []string{oldestRelease, previousRelease, newestRelease}, secondStep},
		{"an untagged older step is an earlier release", []string{constants.EmptyString, newestRelease}, firstStep},
		{"an untagged newest step allows no rollback", []string{previousRelease, newestRelease, constants.EmptyString}, lastStep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, config(none))
			var ran []int64

			if err := migrations.Run(releaseSteps(&ran, tt.releases...)); err != nil {
				t.Fatal(err)
			}

			steps := int64(len(tt.releases))
			if got, want := recordedSteps(t, client), records(tt.want, ran...); !slices.Equal(got, want) {
				t.Errorf("records written = %v, want %v", got, want)
			}
			if got, want := storedRecord(t, client), (migrationRecord{steps, tt.want}); got != want {
				t.Errorf("stored record = %v, want %v", got, want)
			}
		})
	}
}

// A rollback keeps the rollback step, so a second rollback still refuses, and the next upgrade
// runs the newer release's steps again.
func TestRunAllowsOneReleaseBack(t *testing.T) {
	client := installFake(t, config(none))
	var ran []int64
	newest := releaseSteps(&ran, oldestRelease, previousRelease, newestRelease)
	previous, oldest := newest[:secondStep], newest[:firstStep]
	upgraded := migrationRecord{lastStep, secondStep}

	if err := migrations.Run(newest); err != nil {
		t.Fatal(err)
	}
	if got := storedRecord(t, client); got != upgraded {
		t.Fatalf("after the upgrade: stored record = %v, want %v", got, upgraded)
	}

	ran = nil
	if err := migrations.Run(previous); err != nil {
		t.Fatalf("one release back: %v", err)
	}
	rolledBack := migrationRecord{secondStep, secondStep}
	if got := storedRecord(t, client); len(ran) != constants.DefaultInitValue || got != rolledBack {
		t.Fatalf("one release back: ran %v and stored %v, want nothing run and %v", ran, got, rolledBack)
	}

	if err := migrations.Run(oldest); err == nil {
		t.Fatal("two releases back started")
	}
	if got := storedRecord(t, client); len(ran) != constants.DefaultInitValue || got != rolledBack {
		t.Fatalf("two releases back: ran %v and stored %v, want nothing run and %v", ran, got, rolledBack)
	}

	if err := migrations.Run(newest); err != nil {
		t.Fatal(err)
	}
	if got := storedRecord(t, client); !slices.Equal(ran, []int64{lastStep}) || got != upgraded {
		t.Errorf("upgraded again: ran %v and stored %v, want [%d] and %v", ran, got, lastStep, upgraded)
	}
}

// A run after a failed one resumes at the failed step.
func TestRunResumesAtTheFailedStep(t *testing.T) {
	client := installFake(t, config(none))
	var ran []int64
	if err := migrations.Run(numberedSteps(lastStep, secondStep, &ran)); err == nil {
		t.Fatal("Run with a failing step returned no error")
	}

	ran = nil
	if err := migrations.Run(numberedSteps(lastStep, noFailure, &ran)); err != nil {
		t.Fatal(err)
	}
	if want := []int64{secondStep, lastStep}; !slices.Equal(ran, want) {
		t.Errorf("steps run after the failure = %v, want %v", ran, want)
	}
	if got, want := storedRecord(t, client), (migrationRecord{lastStep, noPreviousSteps}); got != want {
		t.Errorf("stored record = %v, want %v", got, want)
	}
}

// An older CRD prunes the record and the API server still answers 200: the runner must
// notice, or every start would run the steps again and the guard would never fire.
func TestRunFailsWhenTheSchemaDropsTheRecord(t *testing.T) {
	tests := []struct {
		name     string
		releases []string
		kept     migrationRecord
	}{
		{"both fields dropped", []string{newestRelease}, none},
		{"the rollback step dropped", []string{previousRelease, newestRelease}, migrationRecord{firstStep, noRecord}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, config(none))
			client.PrependReactor(verbPatch, md.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, config(tt.kept), nil
			})
			var ran []int64
			if err := migrations.Run(releaseSteps(&ran, tt.releases...)); err == nil {
				t.Fatal("Run returned no error although the record was dropped")
			}
		})
	}
}

func TestRunFailsWithoutTelarkConfig(t *testing.T) {
	installFake(t)
	var ran []int64
	if err := migrations.Run(numberedSteps(firstStep, noFailure, &ran)); err == nil {
		t.Fatal("Run returned no error without a TelarkConfig")
	}
	if len(ran) != constants.DefaultInitValue {
		t.Errorf("steps run = %v, want none", ran)
	}
}

// The record names a step by its number, so the shipped list counts up from 1 with no gap.
func TestStepsAreNumberedInOrder(t *testing.T) {
	for i, step := range migrations.Steps {
		if want := int64(i + constants.DefaultIncrementValue); step.Number != want {
			t.Errorf("step %q is numbered %d, want %d", step.Name, step.Number, want)
		}
	}
}

// The API server prunes what a schema does not declare, and the runner writes through /status.
func TestCRDDeclaresTheRecord(t *testing.T) {
	raw, err := os.ReadFile(filepath.Clean(crdFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := slices.DeleteFunc(strings.Split(string(raw), lineBreak), func(line string) bool {
		return strings.Contains(line, templateOpen)
	})
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal([]byte(strings.Join(lines, lineBreak)), &crd); err != nil {
		t.Fatal(err)
	}
	if len(crd.Spec.Versions) == constants.DefaultInitValue {
		t.Fatalf("%s has no versions", crdFile)
	}
	for _, version := range crd.Spec.Versions {
		if version.Subresources == nil || version.Subresources.Status == nil {
			t.Errorf("%s has no status subresource", version.Name)
		}
		for _, name := range []string{constants.FieldLastMigration, constants.FieldRollbackMigration} {
			if field := version.Schema.OpenAPIV3Schema.Properties[constants.FieldStatus].Properties[name]; field.Type != typeInteger {
				t.Errorf("%s: status.%s type = %q, want %q", version.Name, name, field.Type, typeInteger)
			}
		}
	}
}
