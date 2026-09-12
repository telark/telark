package snapshots

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/snapshots"
	"github.com/telark/rest/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sjson "k8s.io/apimachinery/pkg/util/json"
)

const (
	scopeQueryParam      = "scope"
	namespaceQueryParam  = "namespace"
	generationQueryParam = "generation"
)

type Client struct {
	*shared.Client
}

type CreateSnapshotPayload struct {
	ID         string `json:"id"`
	Scope      string `json:"scope"`
	Namespace  string `json:"namespace"`
	Generation int    `json:"generation"`
	Manifest   any    `json:"manifest"`
}

type SnapshotStorageInfo struct {
	TotalPVCSpace  map[string]any `json:"totalPVCSpace"`
	ConsumedSpace  map[string]any `json:"consumedSpace"`
	AvailableSpace map[string]any `json:"availableSpace"`
	TotalSnapshots int            `json:"totalSnapshots"`
	SnapshotsPath  string         `json:"snapshotsPath"`
	SnapshotScopes []string       `json:"snapshotScopes"`
	PVCName        string         `json:"pvcName"`
	PVCNamespace   string         `json:"pvcNamespace"`
}

func NewClient() *Client {
	return &Client{
		Client: shared.New(base.Exporter),
	}
}

func NewClientWithConfig(cfg *shared.ClientConfig) *Client {
	return &Client{
		Client: shared.NewWithConfig(base.Exporter, cfg),
	}
}

func (c *Client) CreateSnapshot(payload *CreateSnapshotPayload) *response.GenericResponse {
	return c.Create(eps.CreateSnapshot, payload)
}

func (c *Client) GetSnapshot(id string, scope string) (*map[string]any, error) {
	withQuery := base.Endpoint(string(eps.GetSnapshot) + "?scope=" + url.QueryEscape(scope))
	return shared.GetTyped[map[string]any](byID(c.Client, id), withQuery)
}

func (c *Client) GetSnapshotManifest(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation string,
) ([]unstructured.Unstructured, error) {
	_ = ctx // reserved for future context-aware HTTP calls
	epWithQuery, err := appendSnapshotQuery(eps.GetSnapshotManifest, scope, namespace, generation)
	if err != nil {
		return nil, err
	}

	raw, err := shared.GetRawJSONWithHeaders[json.RawMessage](
		byID(c.Client, snapshotID),
		epWithQuery,
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		return nil, err
	}

	// encoding/json decodes every number as float64, which unstructured's typed
	// accessors reject. k8sjson decodes whole numbers as int64, the only numeric
	// form unstructured.Unstructured.Object is allowed to hold.
	var objs []map[string]any
	if err := k8sjson.Unmarshal(*raw, &objs); err != nil {
		return nil, fmt.Errorf(string(errors.ErrRestDecodeResponse), err)
	}

	out := make([]unstructured.Unstructured, constants.EmptySliceLength, len(objs))
	for _, obj := range objs {
		out = append(out, unstructured.Unstructured{Object: obj})
	}
	return out, nil
}

func (c *Client) GetSnapshotInfos() (*SnapshotStorageInfo, error) {
	return shared.GetTyped[SnapshotStorageInfo](c.Client, eps.GetSnapshotInfos)
}

func (c *Client) DeleteSnapshot(
	id string,
	scope string,
	namespace string,
	generation string,
) (*response.GenericResponse, error) {
	if generation == constants.EmptyString {
		return nil, fmt.Errorf(string(errors.ErrRestRequiredParam), generationQueryParam)
	}
	epWithQuery, err := appendSnapshotQuery(eps.DeleteSnapshot, scope, namespace, generation)
	if err != nil {
		return nil, err
	}
	return byID(c.Client, id).Delete(epWithQuery), nil
}

func byID(c *shared.Client, id string) *shared.Client {
	return c.WithParams(map[string]string{constants.IDParam: id})
}

func appendSnapshotQuery(ep base.Endpoint, scope, namespace, generation string) (base.Endpoint, error) {
	q := url.Values{}
	if scope != constants.EmptyString {
		q.Set(scopeQueryParam, scope)
	}
	if namespace != constants.EmptyString {
		q.Set(namespaceQueryParam, namespace)
	}
	if generation != constants.EmptyString {
		if _, err := strconv.Atoi(generation); err != nil {
			return "", fmt.Errorf("invalid generation query param: %v", err)
		}
		q.Set(generationQueryParam, generation)
	}
	if len(q) == constants.EmptySliceLength {
		return ep, nil
	}
	return base.Endpoint(string(ep) + "?" + q.Encode()), nil
}
