package informers

import (
	"context"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type pendingSnapshots struct {
	Generation  int                                    `json:"gen"`
	ContentHash string                                 `json:"hash"`
	Snapshots   []applicationmodel.ApplicationSnapshot `json:"snaps"`
}

type snapshotManifestFn func(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation int,
) ([]unstructured.Unstructured, error)

type reconcilePass struct {
	pipe     redis.Pipeliner
	appName  string
	live     map[string]string
	recorded map[string]string
	postGen  int
}
