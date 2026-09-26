package membership

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"

	metadatabase "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/concurrency"
	"github.com/telark/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// One pass at boot over the records written before both sides were kept in
// step: two lists and one patch per drifted record. The user side is what
// grants, so groups are aligned to it and nobody's access changes.
// ponytail: two replicas booting together both run it; the patches are idempotent.
func Reconcile(ctx context.Context, optimizer *performance.Optimizer) {
	userGroups, err := listSide(metadata.UserAsResourceMetadata, constants.FieldAssignedGroupsIDs)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrMembershipReconcileFailed), err))
		return
	}
	groupMembers, err := listSide(metadata.GroupAsResourceMetadata, constants.FieldAssignedUsersIDs)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrMembershipReconcileFailed), err))
		return
	}

	users, groups := Plan(userGroups, groupMembers)
	apply(ctx, optimizer, metadata.UserAsResourceMetadata, constants.FieldAssignedGroupsIDs, users)
	apply(ctx, optimizer, metadata.GroupAsResourceMetadata, constants.FieldAssignedUsersIDs, groups)
	if len(users) > constants.DefaultInitValue {
		authz.BumpGeneration(ctx)
	}
	lg.Info(fmt.Sprintf(string(constants.InfMembershipReconciled), len(users), len(groups)))
}

// Plan returns, per record that needs it, the list to store. Users keep their
// groups (duplicates and groups that no longer exist dropped); each group then
// lists exactly the users that name it.
func Plan(userGroups, groupMembers map[string][]string) (users, groups map[string][]string) {
	users = map[string][]string{}
	kept := make(map[string][]string, len(userGroups))
	for id, refs := range userGroups {
		list := slices.DeleteFunc(resourcesshared.DedupeIDs(slices.Clone(refs)), func(ref string) bool {
			_, exists := groupMembers[ref]
			return !exists
		})
		kept[id] = list
		if !slices.Equal(list, refs) {
			users[id] = list
		}
	}

	groups = map[string][]string{}
	userIDs := slices.Sorted(maps.Keys(kept))
	for id, members := range groupMembers {
		list := slices.DeleteFunc(resourcesshared.DedupeIDs(slices.Clone(members)), func(user string) bool {
			return !slices.Contains(kept[user], id)
		})
		for _, user := range userIDs {
			if slices.Contains(kept[user], id) && !slices.Contains(list, user) {
				list = append(list, user)
			}
		}
		if !slices.Equal(list, members) {
			groups[id] = list
		}
	}
	return users, groups
}

// Terminating records are left to the cleanup cascade, so they count as absent.
func listSide(md metadatabase.Metadata, field string) (map[string][]string, error) {
	result := api.ListCustomResources(md)
	if result.Status != http.StatusOK {
		return nil, result.Error
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}

	side := make(map[string][]string, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		if item.GetDeletionTimestamp() != nil {
			continue
		}
		refs, _, _ := unstructured.NestedStringSlice(item.Object, constants.SpecField, field)
		side[item.GetName()] = refs
	}
	return side, nil
}

func apply(ctx context.Context, optimizer *performance.Optimizer, md metadatabase.Metadata, field string, patches map[string][]string) {
	for id, list := range patches {
		lock := concurrency.GetLock(id)
		lock.Lock()
		err := patchList(ctx, optimizer, md, field, id, list)
		lock.Unlock()
		if err != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrMembershipReconcilePatch), md.Kind, id, err))
		}
	}
}
