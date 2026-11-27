package authrequests

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	lg          = constants.GetLogger(constants.PrefixAuthRequests)
	ErrNotFound = errors.New(string(constants.ErrAuthRequestNotFound))
	ErrPhase    = errors.New(string(constants.ErrAuthRequestPhase))
	reported    = []string{
		authrequest.PhaseWaitingForApproval, authrequest.PhaseRecovering, authrequest.PhaseLinkReady,
		authrequest.PhaseCompleted, authrequest.PhaseDenied, authrequest.PhaseCanceled, authrequest.PhaseExpired,
		authrequest.PhaseFailed,
	}
)

// The requests no run has finished, oldest first: the ones not started yet, and the ones a run left halfway. The mirror
// trails a status write, so each one is read back: a run that just ended is never handed to the next.
func Pending() ([]authrequest.AuthRequest, error) {
	items, err := list()
	if err != nil {
		return nil, err
	}
	pending := make([]authrequest.AuthRequest, constants.DefaultInitValue, len(items))
	for i := range items {
		if !unfinished(toRequest(&items[i])) {
			continue
		}
		result := api.GetCustomResourceByName(items[i].GetName(), metadata.AuthRequestMetadata)
		stored, ok := result.Data.(*unstructured.Unstructured)
		if k8serrors.IsNotFound(result.Error) {
			continue
		}
		if !ok {
			return nil, fmt.Errorf(string(constants.ErrAuthRequestList), result.Error)
		}
		if request := toRequest(stored); unfinished(request) {
			pending = append(pending, request)
		}
	}
	slices.SortFunc(pending, func(a, b authrequest.AuthRequest) int {
		return cmp.Or(cmp.Compare(a.CreatedAt, b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return pending, nil
}

func unfinished(request authrequest.AuthRequest) bool {
	return request.Phase == constants.EmptyString || request.Phase == authrequest.PhasePending || request.InFlight()
}

// The API server answers 200 even when it drops a field its stored CRD lacks, so the phase that came back must be
// the one written.
func SetStatus(name string, status authrequest.Status) error {
	if !slices.Contains(reported, status.Phase) {
		return ErrPhase
	}
	result := api.PatchCustomResourceStatus(metadata.AuthRequestMetadata, name, map[string]any{constants.FieldStatus: status})
	if k8serrors.IsNotFound(result.Error) {
		return ErrNotFound
	}
	if result.Error != nil {
		return fmt.Errorf(string(constants.ErrAuthRequestStatusWrite), name, result.Error)
	}
	stored, ok := result.Data.(*unstructured.Unstructured)
	if !ok || toRequest(stored).Phase != status.Phase {
		return fmt.Errorf(string(constants.ErrAuthRequestStatusPruned), name)
	}
	request := toRequest(stored)
	lg.Info(fmt.Sprintf(string(constants.InfAuthRequestStatus), name, request.Email, request.RequestedBy, status.Phase))
	return nil
}

func list() ([]unstructured.Unstructured, error) {
	if items, synced := informers.ListAuthRequests(); synced {
		return items, nil
	}
	result := api.ListCustomResources(metadata.AuthRequestMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrAuthRequestList), result.Error)
	}
	listed, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok || listed == nil {
		return nil, fmt.Errorf(string(constants.ErrAuthRequestList), constants.ErrInvalidResourceTypeReturned)
	}
	return listed.Items, nil
}

func toRequest(obj *unstructured.Unstructured) authrequest.AuthRequest {
	text := func(fields ...string) string {
		value, _, _ := unstructured.NestedString(obj.Object, fields...)
		return value
	}
	return authrequest.AuthRequest{
		ID: obj.GetName(),
		Spec: authrequest.Spec{
			Email:        text(constants.SpecField, authrequest.FieldEmail),
			RequestedBy:  text(constants.SpecField, authrequest.FieldRequestedBy),
			RecipientKey: text(constants.SpecField, authrequest.FieldRecipientKey),
		},
		Status:    authrequest.Status{Phase: text(constants.FieldStatus, metadata.StatusPhase)},
		CreatedAt: obj.GetCreationTimestamp().UTC().Format(time.RFC3339),
	}
}
