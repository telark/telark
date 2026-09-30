package membership

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	metadatabase "github.com/telark/telark/internal/data/metadata/base"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixMain)

// Membership is stored on both sides and grants read the user side. The
// counterparts are written before the caller's own record, so a failure
// leaves that record untouched and the same request can simply be retried:
// every write here is a no-op once applied.
func MirrorUserGroups(ctx context.Context, optimizer *performance.Optimizer, userID string, added, removed []string) error {
	return mirror(ctx, optimizer, metadata.GroupMetadata, constants.FieldUserRefs, userID, added, removed)
}

func MirrorGroupMembers(ctx context.Context, optimizer *performance.Optimizer, groupID string, added, removed []string) error {
	return mirror(ctx, optimizer, metadata.UserMetadata, constants.FieldGroupRefs, groupID, added, removed)
}

func mirror(ctx context.Context, optimizer *performance.Optimizer, md metadatabase.Metadata, field, member string, added, removed []string) error {
	for _, id := range added {
		if err := setMember(ctx, optimizer, md, field, id, member, true); err != nil {
			return err
		}
	}
	for _, id := range removed {
		if err := setMember(ctx, optimizer, md, field, id, member, false); err != nil {
			return err
		}
	}
	return nil
}

// One counterpart lock at a time, never nested with the caller's own: the user
// and group handlers would otherwise take the two locks in opposite orders.
func setMember(ctx context.Context, optimizer *performance.Optimizer, md metadatabase.Metadata, field, id, member string, present bool) error {
	lock := concurrency.GetLock(id)
	lock.Lock()
	defer lock.Unlock()

	result := api.GetCustomResourceByName(id, md)
	switch status := sharedutils.StatusForResult(result); status {
	case http.StatusOK:
	case http.StatusNotFound:
		// The counterpart is gone: there is nothing left to keep consistent.
		return nil
	default:
		return fmt.Errorf(string(constants.ErrMembershipMirrorFailed), id, result.Error)
	}

	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf(string(constants.ErrMembershipMirrorFailed), id, errors.New(string(constants.ErrInvalidResourceTypeReturned)))
	}
	current, _, _ := unstructured.NestedStringSlice(resource.Object, constants.SpecField, field)
	next, changed := WithMember(current, member, present)
	if !changed {
		return nil
	}
	return patchList(ctx, optimizer, md, field, id, next)
}

func patchList(ctx context.Context, optimizer *performance.Optimizer, md metadatabase.Metadata, field, id string, list []string) error {
	spec := map[string]any{field: list}
	resourcesshared.AddLastUpdateDateToPatchBody(spec)
	patch := sharedutils.PatchCustomResource(md, id, map[string]any{constants.SpecField: spec})
	if patch.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrMembershipMirrorFailed), id, patch.Error)
	}

	invalidate(ctx, optimizer, md, id)
	return nil
}

func WithMember(current []string, member string, present bool) (next []string, changed bool) {
	if slices.Contains(current, member) == present {
		return current, false
	}
	if present {
		return append(slices.Clone(current), member), true
	}
	return slices.DeleteFunc(slices.Clone(current), func(id string) bool { return id == member }), true
}

func invalidate(ctx context.Context, optimizer *performance.Optimizer, md metadatabase.Metadata, id string) {
	if md.Kind == metadata.UserMetadata.Kind {
		userutils.InvalidateUserCaches(optimizer, id)
		authz.ForgetUserGrants(ctx, id)
		return
	}
	resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceGroup, id)
}
