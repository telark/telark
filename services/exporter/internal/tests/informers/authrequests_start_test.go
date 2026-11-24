package informers

import (
	"testing"

	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
)

const envKubernetesServiceHost = "KUBERNETES_SERVICE_HOST"

// Off cluster and without a client the mirror never starts: the call returns and reads keep asking the apiserver.
func TestStartAuthRequestsWithoutAClientServesNothing(t *testing.T) {
	t.Setenv(envKubernetesServiceHost, constants.EmptyString)
	k8sclient.SetDynamicClient(nil)

	informers.StartAuthRequests(t.Context())

	if items, synced := informers.ListAuthRequests(); synced || items != nil || informers.AuthRequestsSynced() {
		t.Fatal("nothing must be served when the informer could not start")
	}
}
