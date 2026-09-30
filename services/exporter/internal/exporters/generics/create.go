package generics

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/crds/api"
	kubeshared "github.com/telark/telark/internal/kcore/shared"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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
	result := createWithQuotaRetry(template, md)

	if result.Status == http.StatusConflict {
		responseutils.LogAndSendResponse(w, http.StatusConflict, response.OperationAlreadyExists, string(globalerrors.ErrResExists), nil, nil)
		return
	}

	if result.Status != http.StatusOK {
		// kcore stamps every failure 500, which would hide a schema rejection (422) behind a
		// server error; the Kubernetes status carries the real code.
		errorMsg := sharedutils.GenerateResourceError(globalerrors.ErrCreateRes, name, result.Error)
		status := sharedutils.StatusForResult(result)
		sharedutils.LogByStatusAndSend(w, status, response.OperationError, errorMsg, nil, result.Error)
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

// The slot is released with defer: a panic between acquire and release would retire it for the
// process lifetime, and k8sCreatePoolSize of those deadlock every CRD create.
func createWithQuotaRetry(
	template *unstructured.Unstructured,
	md metadata.Metadata,
) kubeshared.KubernetesAPIData {
	k8sCreateSem <- struct{}{}
	defer func() { <-k8sCreateSem }()

	result := api.CreateCustomResourceWithStatus(template, md)
	for attempt := constants.DefaultIncrementValue; attempt < maxCreateAttempts &&
		result.Status != http.StatusOK && result.Error != nil &&
		strings.Contains(result.Error.Error(), quotaTimeoutMsg); attempt++ {
		time.Sleep(createRetryDelay)
		result = api.CreateCustomResourceWithStatus(template, md)
	}
	return result
}
