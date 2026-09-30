package crds

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var widgetGVR = schema.GroupVersionResource{Group: TestGroup, Version: TestVersion, Resource: TestPlural}

func failingReactor(err error) k8stesting.ReactionFunc {
	return func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, err }
}

func deploymentGetError(reactErr error) error {
	client := kubefake.NewClientset()
	if reactErr != nil {
		client.PrependReactor(TestVerbGet, TestResource, failingReactor(reactErr))
	}
	_, err := client.AppsV1().Deployments(TestNamespace).Get(context.Background(), TestName, k8smetav1.GetOptions{})
	return err
}

func widgetGetError(reactErr error) error {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{widgetGVR: TestListKind})
	if reactErr != nil {
		client.PrependReactor(TestVerbGet, TestPlural, failingReactor(reactErr))
	}
	_, err := client.Resource(widgetGVR).Namespace(TestNamespace).Get(context.Background(), TestName, k8smetav1.GetOptions{})
	return err
}

func TestExistsFromGetError(t *testing.T) {
	gr := schema.GroupResource{Group: TestGroup, Resource: TestPlural}
	forbidden := k8serrors.NewForbidden(gr, TestName, errors.New(TestName))
	timeout := k8serrors.NewServerTimeout(gr, TestVerbGet, TestTimeoutS)
	throttled := k8serrors.NewTooManyRequests(TestName, TestTimeoutS)

	cases := []struct {
		name       string
		err        error
		wantExists bool
		wantErr    error
	}{
		{"found", nil, true, nil},
		{"deployment not found", deploymentGetError(nil), false, nil},
		{"deployment forbidden", deploymentGetError(forbidden), false, forbidden},
		{"deployment throttled", deploymentGetError(throttled), false, throttled},
		{"custom resource not found", widgetGetError(nil), false, nil},
		{"custom resource timeout", widgetGetError(timeout), false, timeout},
	}
	for _, c := range cases {
		exists, err := shared.ExistsFromGetError(c.err)
		if exists != c.wantExists || !errors.Is(err, c.wantErr) {
			t.Errorf(ExpectedExistsFromGetError, c.name, exists, err, c.wantExists, c.wantErr)
		}
	}
}

// An empty name is rejected before any client is built, so no cluster is needed.
func TestEmptyNameIsBadRequest(t *testing.T) {
	md := base.Metadata{BaseGroup: TestGroup, Version: TestVersion, Plural: TestPlural, Namespace: TestNamespace, Kind: TestKind}
	results := map[string]shared.KubernetesAPIData{
		"get":    api.GetCustomResourceByName(constants.EmptyString, md),
		"delete": api.DeleteCustomResourceByName(constants.EmptyString, md),
		"patch":  api.PatchCustomResource(md, constants.EmptyString, map[string]any{TestName: TestName}),
		"update": api.UpdateCustomResource(constants.EmptyString, md, &unstructured.Unstructured{}),
	}
	for op, r := range results {
		if r.Status != http.StatusBadRequest || !k8serrors.IsBadRequest(r.Error) {
			t.Errorf(ExpectedBadRequest, op, r.Status, r.Error, http.StatusBadRequest)
		}
	}

	exists, err := api.CheckCustomResourceExistsByName(constants.EmptyString, md)
	if exists || !k8serrors.IsBadRequest(err) {
		t.Errorf(ExpectedBadRequest, "check", http.StatusBadRequest, err, http.StatusBadRequest)
	}
}
