package shared

import (
	"net/http"

	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/exporters/generics"
	"github.com/telark/exporter/utils/concurrency"
)

func DeleteResource(w http.ResponseWriter, _ *http.Request, resourceMetadata metadata.Metadata, resourceName string) {
	lock := concurrency.GetLock(resourceName)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericDeleteCustomResource(w, resourceMetadata, resourceName)
}
