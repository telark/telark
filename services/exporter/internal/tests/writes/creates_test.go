package writes

import (
	"context"
	"net/http"
	"testing"

	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	usersPath  = "/api/v1/users"
	newUser    = `{"username":"new","email":"test@example.com","creationDate":"2026-10-16T00:00:00Z",` + activeStatus
	markedUser = `{"username":"marked","email":"test2@example.com","creationDate":"2026-10-16T00:00:00Z","bootstrap":true,` +
		activeStatus
	activeStatus = `"status":{"phase":"active"}}`
	keyBootstrap = "bootstrap"
	newGroup     = `{"name":"operators"}`
	newRole      = `{"name":"readers","description":"Reads applications","categoryRef":"cat-00001-0001-0005","status":"active",` +
		`"scopesAndPermissions":[{"scope":"applications","level":"readonly"}]}`
)

// A create template carries its finalizers as a []string, which the API server reads as JSON but the fake can't deep
// copy, so creates reach the fake as JSON too.
type jsonCreates struct{ dynamic.Interface }

type jsonResource struct {
	dynamic.NamespaceableResourceInterface
}

type jsonNamespace struct{ dynamic.ResourceInterface }

func (c jsonCreates) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return jsonResource{c.Interface.Resource(gvr)}
}

func (r jsonResource) Namespace(namespace string) dynamic.ResourceInterface {
	return jsonNamespace{r.NamespaceableResourceInterface.Namespace(namespace)}
}

func (n jsonNamespace) Create(
	ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string,
) (*unstructured.Unstructured, error) {
	raw, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	sent := &unstructured.Unstructured{}
	if err := sent.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	return n.ResourceInterface.Create(ctx, sent, options, subresources...)
}

// The service creates each record with its cleanup finalizer.
func TestCreatesLand(t *testing.T) {
	cases := []struct {
		name, path, body string
		md               base.Metadata
	}{
		{"user", usersPath, newUser, v1alpha1.UserMetadata},
		{"group", groupsPath, newGroup, v1alpha1.GroupMetadata},
		{"role", rolesPath, newRole, v1alpha1.AccessRoleMetadata},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t)
			k8sclient.SetDynamicClient(jsonCreates{client})
			w := serveAs(t, xauthz.Identity{Internal: true}, http.MethodPost, c.path, c.body)
			if w.Code != http.StatusOK {
				t.Fatalf("create = %d %s", w.Code, w.Body.String())
			}
			list, err := client.Resource(gvrOf(c.md)).Namespace(c.md.Namespace).List(t.Context(), metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Items) != constants.DefaultIncrementValue || len(list.Items[firstItem].GetFinalizers()) != constants.DefaultIncrementValue {
				t.Fatalf("stored = %+v, want one record with its finalizer", list.Items)
			}
		})
	}
}

// Before any pin, a marked account the service creates takes the marker off every other account.
func TestAMarkedCreateBeforeThePinMovesTheMarker(t *testing.T) {
	config := object(v1alpha1.TelarkConfigMetadata, constants.TelarkConfigResourceName, map[string]any{}, nil)
	client := installFake(t, crSeed(v1alpha1.TelarkConfigMetadata, config), adminUser(t, bootstrapUser, nil, nil, true))
	k8sclient.SetDynamicClient(jsonCreates{client})

	if w := serveAs(t, xauthz.Identity{Internal: true}, http.MethodPost, usersPath, markedUser); w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	md := v1alpha1.UserMetadata
	previous, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).Get(t.Context(), bootstrapUser, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if marked, _, _ := unstructured.NestedBool(previous.Object, keySpec, keyBootstrap); marked {
		t.Fatal("the earlier account kept the marker")
	}
}
