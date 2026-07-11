package generics

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

const (
	quotaTimeoutMsg   = "quota evaluation timed out"
	maxCreateAttempts = 2
	createRetryDelay  = 200 * time.Millisecond
	k8sCreatePoolSize = 10
)

// k8sCreateSem limits concurrent CRD creation calls to prevent K8s API overload.
var (
	k8sCreateSem = make(chan struct{}, k8sCreatePoolSize)
	lg           = constants.GetLogger(constants.PrefixGenerics)
)

func GenericCreateCustomResourceWithFinalizers(
	w http.ResponseWriter,
	md metadata.Metadata,
	name string,
	spec map[string]any,
	finalizers []string,
) {
	runCreateFromTemplate(w, md, name, sharedutils.ConvertToCRDTemplateWithFinalizers(md, name, spec, finalizers))
}

func GenericCreateCustomResource(w http.ResponseWriter, md metadata.Metadata, name string, spec map[string]any) {
	runCreateFromTemplate(w, md, name, sharedutils.ConvertToCRDTemplate(md, name, spec))
}

func runCreateFromTemplate(
	w http.ResponseWriter,
	md metadata.Metadata,
	name string,
	template *unstructured.Unstructured,
) {
	k8sCreateSem <- struct{}{}
	result := api.CreateCustomResource(template, md)
	for attempt := constants.DefaultIncrementValue; attempt < maxCreateAttempts &&
		result.Status != http.StatusOK && result.Error != nil &&
		strings.Contains(result.Error.Error(), quotaTimeoutMsg); attempt++ {
		time.Sleep(createRetryDelay)
		result = api.CreateCustomResource(template, md)
	}
	<-k8sCreateSem

	if result.Status == http.StatusConflict {
		responseutils.LogAndSendResponse(w, http.StatusConflict, response.OperationAlreadyExists, string(globalerrors.ErrResExists), nil, nil)
		return
	}

	if result.Status != http.StatusOK {
		errorMsg := sharedutils.GenerateResourceError(globalerrors.ErrCreateRes, name, result.Error)
		responseutils.LogAndSendResponse(w, result.Status, response.OperationError, errorMsg, nil, result.Error)
		return
	}

	created, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(globalerrors.ErrGetRes),
			nil,
			errors.New(string(constants.ErrInvalidResourceTypeReturned)),
		)
		return
	}

	filterAndRespond(w, created, messages.SuccessCreateRes)
}
