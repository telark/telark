package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/telark/discovery/internal/circuitbreaker"
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
		if circuitbreaker.IsOpen(err) {
			return constants.EmptyString, err
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
	path, err := c.createSnapshotCall(id, scope, namespace, generation, manifest)
	if err != nil {
		return constants.EmptyString, err
	}
	if err := c.verifySnapshotReadable(id, scope, namespace, generation); err != nil {
		return constants.EmptyString, err
	}
	return path, nil
}

func (c *SnapshotClient) createSnapshotCall(
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
	var path string
	err := circuitbreaker.ExecuteExporter(func() error {
		resp := c.client.CreateSnapshot(payload)
		if resp == nil {
			return errors.New(string(constants.ErrFailedCreateSnapshot))
		}
		if resp.Status != http.StatusOK {
			return classifyStatus(resp.Status, fmt.Errorf(
				string(constants.ErrFailedCreateSnapshotWithStatus),
				resp.Status,
				resp.Message,
			))
		}
		var readErr error
		path, readErr = snapshotPathFrom(resp.Data)
		return readErr
	})
	return path, err
}

// A malformed payload is the exporter answering, not failing to answer, so it
// must not count towards the breaker.
func snapshotPathFrom(data any) (string, error) {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return constants.EmptyString, circuitbreaker.NotCounted(
			errors.New(string(constants.ErrFailedGetSnapshotDataMap)),
		)
	}
	path, ok := dataMap[constants.SnapshotPathKey].(string)
	if !ok || path == constants.EmptyString {
		return constants.EmptyString, circuitbreaker.NotCounted(
			errors.New(string(constants.ErrFailedGetSnapshotPath)),
		)
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
		if circuitbreaker.IsOpen(err) {
			return err
		}
		lastErr = err
		if attempt < snapshotVerifyAttempts {
			time.Sleep(snapshotVerifyDelay)
		}
	}
	return fmt.Errorf(string(constants.ErrFailedGetSnapshotManifest), id, lastErr)
}

func (c *SnapshotClient) DeleteSnapshot(
	id string,
	scope string,
	namespace string,
	generation int,
) error {
	return circuitbreaker.ExecuteExporter(func() error {
		resp, err := c.client.DeleteSnapshot(id, scope, namespace, strconv.Itoa(generation))
		if err != nil {
			return classifyWrapped(err, fmt.Errorf(string(constants.ErrFailedDeleteSnapshot), err))
		}
		if resp == nil {
			return errors.New(string(constants.ErrFailedDeleteSnapshot))
		}
		if resp.Status != http.StatusOK {
			return classifyStatus(resp.Status, fmt.Errorf(
				string(constants.ErrFailedDeleteSnapshotWithStatus),
				resp.Status,
				resp.Message,
			))
		}
		return nil
	})
}

func (c *SnapshotClient) GetSnapshotManifest(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation int,
) ([]unstructured.Unstructured, error) {
	var out []unstructured.Unstructured
	err := circuitbreaker.ExecuteExporter(func() error {
		result, callErr := c.client.GetSnapshotManifest(
			ctx,
			snapshotID,
			scope,
			namespace,
			strconv.Itoa(generation),
		)
		if callErr != nil {
			return classifyWrapped(callErr, fmt.Errorf(
				string(constants.ErrFailedGetSnapshotManifest), snapshotID, callErr,
			))
		}
		out = result
		return nil
	})
	return out, err
}
