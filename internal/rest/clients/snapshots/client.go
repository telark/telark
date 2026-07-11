package snapshots

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/constants"
	eps "github.com/telark/rest/endpoints/snapshots"
	"github.com/telark/rest/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetSnapshot),
		constants.IDParam,
		id,
	)

	withQuery := base.Endpoint(string(ep) + "?scope=" + url.QueryEscape(scope))
	return shared.GetTyped[map[string]any](c.Client, withQuery)
}

func (c *Client) GetSnapshotManifest(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation string,
) ([]unstructured.Unstructured, error) {
	_ = ctx // reserved for future context-aware HTTP calls
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetSnapshotManifest),
		constants.IDParam,
		snapshotID,
	)
	epWithQuery, err := appendManifestQuery(ep, scope, namespace, generation)
	if err != nil {
		return nil, err
	}

	objs, err := shared.GetRawJSONWithHeaders[[]map[string]any](
		c.Client,
		epWithQuery,
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		return nil, err
	}

	out := make([]unstructured.Unstructured, constants.EmptySliceLength, len(*objs))
	for _, obj := range *objs {
		out = append(out, unstructured.Unstructured{Object: obj})
	}
	return out, nil
}

func (c *Client) GetSnapshotInfos() (*SnapshotStorageInfo, error) {
	return shared.GetTyped[SnapshotStorageInfo](c.Client, eps.GetSnapshotInfos)
}

func appendManifestQuery(ep base.Endpoint, scope, namespace, generation string) (base.Endpoint, error) {
	q := url.Values{}
	if scope != constants.EmptyString {
		q.Set("scope", scope)
	}
	if namespace != constants.EmptyString {
		q.Set("namespace", namespace)
	}
	if generation != constants.EmptyString {
		if _, err := strconv.Atoi(generation); err != nil {
			return "", fmt.Errorf("invalid generation query param: %v", err)
		}
		q.Set("generation", generation)
	}
	if len(q) == constants.EmptySliceLength {
		return ep, nil
	}
	return base.Endpoint(string(ep) + "?" + q.Encode()), nil
}
