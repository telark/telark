package rollback

import (
	"time"

	"github.com/telark/discovery/internal/clients"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	kubeClient     *kubernetes.Clientset
	dyn            dynamic.Interface
	mapper         meta.RESTMapper
	snapshotClient *clients.SnapshotClient
	notifClient    *clients.NotificationClient
}

type rollbackPatchOpts struct {
	Status      string
	ErrorMsg    string
	CompletedAt *time.Time
}
