package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/clients/shared"
	snapshotsclient "github.com/telark/rest/clients/snapshots"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	createSnapshotTimeout  = 5 * time.Second
	snapshotVerifyAttempts = 3
	snapshotVerifyDelay    = 200 * time.Millisecond
)

type SnapshotClient struct {
	client *snapshotsclient.Client
}

func NewSnapshotClient() *SnapshotClient {
	cfg := &shared.ClientConfig{Timeout: createSnapshotTimeout}
	return &SnapshotClient{
		client: snapshotsclient.NewClientWithConfig(cfg),
	}
}

func (c *SnapshotClient) CreateSnapshotAndReturnPath(
	id string,
	scope string,
	namespace string,
	generation int,
	manifest any,
) (string, error) {
	attemptMax := config.SnapshotWriteMaxAttempts()
	interval := config.SnapshotWriteRetryInterval()
	var lastErr error
	for attempt := constants.DefaultAddValue; attempt <= attemptMax; attempt++ {
		path, err := c.createSnapshotOnce(id, scope, namespace, generation, manifest)
		if err == nil {
			return path, nil
		}
		lastErr = err
		if attempt < attemptMax {
			time.Sleep(interval)
		}
	}
	return constants.EmptyString, lastErr
}

func (c *SnapshotClient) createSnapshotOnce(
	id string,
	scope string,
	namespace string,
	generation int,
	manifest any,
) (string, error) {
	payload := &snapshotsclient.CreateSnapshotPayload{
		ID:         id,
		Scope:      scope,
		Namespace:  namespace,
		Generation: generation,
		Manifest:   manifest,
	}
	resp := c.client.CreateSnapshot(payload)
	if resp == nil {
		return constants.EmptyString, errors.New(string(constants.ErrFailedCreateSnapshot))
	}
	if resp.Status != http.StatusOK {
		return constants.EmptyString, fmt.Errorf(
			string(constants.ErrFailedCreateSnapshotWithStatus),
			resp.Status,
			resp.Message,
		)
	}
	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrFailedGetSnapshotDataMap))
	}
	path, ok := dataMap["path"].(string)
	if !ok || path == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrFailedGetSnapshotPath))
	}
	if err := c.verifySnapshotReadable(id, scope, namespace, generation); err != nil {
		return constants.EmptyString, err
	}
	return path, nil
}

func (c *SnapshotClient) verifySnapshotReadable(
	id string,
	scope string,
	namespace string,
	generation int,
) error {
	var lastErr error
	for attempt := constants.DefaultAddValue; attempt <= snapshotVerifyAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), createSnapshotTimeout)
		_, err := c.GetSnapshotManifest(ctx, id, scope, namespace, generation)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < snapshotVerifyAttempts {
			time.Sleep(snapshotVerifyDelay)
		}
	}
	return fmt.Errorf(string(constants.ErrFailedGetSnapshotManifest), id, lastErr)
}

func (c *SnapshotClient) GetSnapshotManifest(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation int,
) ([]unstructured.Unstructured, error) {
	out, err := c.client.GetSnapshotManifest(
		ctx,
		snapshotID,
		scope,
		namespace,
		strconv.Itoa(generation),
	)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedGetSnapshotManifest), snapshotID, err)
	}
	return out, nil
}
