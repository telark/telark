package snapshots

import (
	"context"
	"net/url"

	"github.com/plsyro/rest/base"
	"github.com/plsyro/rest/clients/shared"
	"github.com/plsyro/rest/constants"
	eps "github.com/plsyro/rest/endpoints/snapshots"
	"github.com/plsyro/rest/response"
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

// GetSnapshotManifest fetches and returns the sanitized manifest for the given snapshot ID from the exporter-service.
func (c *Client) GetSnapshotManifest(
	ctx context.Context,
	snapshotID string,
) ([]unstructured.Unstructured, error) {
	_ = ctx // reserved for future context-aware HTTP calls
	ep := shared.SubstituteEndpointWithParam(
		string(eps.GetSnapshotManifest),
		constants.IDParam,
		snapshotID,
	)

	objs, err := shared.GetRawJSONWithHeaders[[]map[string]any](
		c.Client,
		ep,
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		return nil, err
	}

	out := make([]unstructured.Unstructured, 0, len(*objs))
	for _, obj := range *objs {
		out = append(out, unstructured.Unstructured{Object: obj})
	}
	return out, nil
}
