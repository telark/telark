package shared

import (
	"net/http"

	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/utils/concurrency"
)

func DeleteResource(w http.ResponseWriter, resourceMetadata metadata.Metadata, resourceName string) {
	lock := concurrency.GetLock(resourceName)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericDeleteCustomResource(w, resourceMetadata, resourceName)
}
