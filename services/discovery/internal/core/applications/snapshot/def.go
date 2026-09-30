package snapshot

import (
	"cmp"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/telark/telark/internal/data/resources/application"
	kcoremanifest "github.com/telark/telark/internal/kcore/manifest"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/shared"
	appshared "github.com/telark/telark/services/discovery/internal/core/applications/shared"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type snapshotResourceItem struct {
	Kind      string
	Name      string
	Namespace string
	Manifest  map[string]any
}

const (
	snapshotIDPrefix    = "snap-"
	snapshotIDSuffixLen = 8
)

var GetManifestFromCache func(kind, name, ns string) (map[string]any, bool)

var snapshotKindApplyOrder = map[string]int{
	appshared.KindServiceAccount:          constants.ManifestOrderServiceAccount,
	appshared.KindConfigMap:               constants.ManifestOrderConfigMap,
	appshared.KindSecret:                  constants.ManifestOrderSecret,
	appshared.KindPersistentVolumeClaim:   constants.ManifestOrderPersistentVolumeClaim,
	appshared.KindService:                 constants.ManifestOrderService,
	appshared.KindNetworkPolicy:           constants.ManifestOrderNetworkPolicy,
	appshared.KindDeployment:              constants.ManifestOrderDeployment,
	appshared.KindStatefulSet:             constants.ManifestOrderStatefulSet,
	appshared.KindDaemonSet:               constants.ManifestOrderDaemonSet,
	appshared.KindJob:                     constants.ManifestOrderJob,
	appshared.KindCronJob:                 constants.ManifestOrderCronJob,
	appshared.KindIngress:                 constants.ManifestOrderIngress,
	appshared.KindHorizontalPodAutoscaler: constants.ManifestOrderHorizontalPodAutoscaler,
	appshared.KindVerticalPodAutoscaler:   constants.ManifestOrderVerticalPodAutoscaler,
}

func BuildSnapshotEntries(
	ctx context.Context,
	createSnapshot func(id, scope, namespace string, generation int, manifestPayload any) (string, error),
	stored *application.Application,
	generation int,
	changeClass string,
	severity string,
	takenAt time.Time,
) []application.ApplicationSnapshot {
	if createSnapshot == nil {
		return []application.ApplicationSnapshot{}
	}
	scope := config.DefaultSnapshotScope()
	byNS := groupResourcesByNamespace(stored.Resources)
	namespaces := sortedNamespaceKeys(byNS)
	out := make([]application.ApplicationSnapshot, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if !shared.IsValidSnapshotSeverity(severity) {
			continue
		}
		items := collectSnapshotItems(ctx, byNS[ns])
		if len(items) == constants.DefaultInitValue {
			continue
		}
		if !hasResolvableManifestItems(items) {
			continue
		}
		slices.SortFunc(items, func(a, b snapshotResourceItem) int {
			return cmp.Compare(kindOrder(a.Kind), kindOrder(b.Kind))
		})
		payload := snapshotManifestPayload(items)
		size, ok := shared.SnapshotPayloadSize(payload)
		if !ok || size <= constants.DefaultInitValue {
			continue
		}
		id, idErr := NewSnapshotID()
		if idErr != nil {
			continue
		}
		path, err := createSnapshot(id, scope, ns, generation, payload)
		if err != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(string(constants.ErrFailedCreateExporterSnapshot), id, ns, err),
			)
			continue
		}
		out = append(out, application.ApplicationSnapshot{
			Generation:  generation,
			ChangeClass: changeClass,
			Severity:    severity,
			TakenAt:     takenAt.UTC().Format(time.RFC3339Nano),
			ID:          id,
			Namespace:   ns,
			Path:        path,
		})
	}
	return out
}

func BuildSnapshotEntriesStrict(
	ctx context.Context,
	createSnapshot func(id, scope, namespace string, generation int, manifestPayload any) (string, error),
	stored *application.Application,
	generation int,
	changeClass string,
	severity string,
	takenAt time.Time,
) ([]application.ApplicationSnapshot, error) {
	if createSnapshot == nil {
		return nil, errors.New(string(constants.ErrSnapshotCreateFuncNil))
	}
	if stored == nil {
		return nil, errors.New(string(constants.ErrSnapshotStoredApplicationNil))
	}
	scope := config.DefaultSnapshotScope()
	byNS := groupResourcesByNamespace(stored.Resources)
	namespaces := sortedNamespaceKeys(byNS)
	out := make([]application.ApplicationSnapshot, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if !shared.IsValidSnapshotSeverity(severity) {
			continue
		}
		items := collectSnapshotItems(ctx, byNS[ns])
		if len(items) == constants.DefaultInitValue {
			continue
		}
		if !hasResolvableManifestItems(items) {
			continue
		}
		slices.SortFunc(items, func(a, b snapshotResourceItem) int {
			return cmp.Compare(kindOrder(a.Kind), kindOrder(b.Kind))
		})
		payload := snapshotManifestPayload(items)
		size, ok := shared.SnapshotPayloadSize(payload)
		if !ok || size <= constants.DefaultInitValue {
			continue
		}
		id, idErr := NewSnapshotID()
		if idErr != nil {
			return nil, idErr
		}
		path, err := createSnapshot(id, scope, ns, generation, payload)
		if err != nil {
			return nil, fmt.Errorf(string(constants.ErrFailedCreateSnapshot), err)
		}
		out = append(out, application.ApplicationSnapshot{
			Generation:  generation,
			ChangeClass: changeClass,
			Severity:    severity,
			TakenAt:     takenAt.UTC().Format(time.RFC3339Nano),
			ID:          id,
			Namespace:   ns,
			Path:        path,
		})
	}
	return out, nil
}

func hasResolvableManifestItems(items []snapshotResourceItem) bool {
	if len(items) == constants.DefaultInitValue {
		return false
	}
	for i := range items {
		if len(items[i].Manifest) == constants.DefaultInitValue {
			return false
		}
	}
	return true
}

func kindOrder(kind string) int {
	if o, ok := snapshotKindApplyOrder[kind]; ok {
		return o
	}
	return constants.ManifestOrderUnknown
}

func groupResourcesByNamespace(resources []application.Resource) map[string][]application.Resource {
	m := make(map[string][]application.Resource)
	for _, res := range resources {
		m[res.Namespace] = append(m[res.Namespace], res)
	}
	return m
}

func sortedNamespaceKeys(m map[string][]application.Resource) []string {
	keys := make([]string, constants.DefaultInitValue, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func collectSnapshotItems(ctx context.Context, resources []application.Resource) []snapshotResourceItem {
	results := make([]*snapshotResourceItem, len(resources))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.SnapshotItemFetchConcurrency)
	fetchTimeout := config.SnapshotFetchTimeout()
	for i := range resources {
		res := resources[i]
		g.Go(func() error {
			item := fetchSnapshotItem(gctx, res, fetchTimeout)
			if item == nil {
				return nil
			}
			mu.Lock()
			results[i] = item
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	items := make([]snapshotResourceItem, constants.DefaultInitValue, len(resources))
	for _, r := range results {
		if r != nil {
			items = append(items, *r)
		}
	}
	return items
}

func fetchSnapshotItem(
	ctx context.Context,
	res application.Resource,
	fetchTimeout time.Duration,
) *snapshotResourceItem {
	if GetManifestFromCache != nil {
		if m, ok := GetManifestFromCache(res.Kind, res.Name, res.Namespace); ok {
			// Informer objects carry cluster-managed metadata; GetRawManifest
			// already cleans its own result, so only the cache hit needs this.
			kcoremanifest.CleanManifestForApply(m)
			return &snapshotResourceItem{
				Kind:      res.Kind,
				Name:      res.Name,
				Namespace: res.Namespace,
				Manifest:  m,
			}
		}
	}
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	rawManifest, err := kcoremanifest.GetRawManifest(fetchCtx, res.Kind, res.Name, res.Namespace)
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.ErrFailedFetchManifestForSnapshot),
				res.Kind, res.Name, res.Namespace, err),
		)
		return nil
	}
	if rawManifest == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(rawManifest, &m); err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.ErrFailedDecodeManifest),
				res.Kind, res.Name, res.Namespace, err),
		)
		return nil
	}
	delete(m, shared.ManifestStatusKey)
	return &snapshotResourceItem{
		Kind:      res.Kind,
		Name:      res.Name,
		Namespace: res.Namespace,
		Manifest:  m,
	}
}

func snapshotManifestPayload(items []snapshotResourceItem) map[string]any {
	resourcePayload := make([]map[string]any, constants.DefaultInitValue, len(items))
	for _, item := range items {
		resourcePayload = append(resourcePayload, map[string]any{
			shared.PayloadKeyKind:      item.Kind,
			shared.PayloadKeyName:      item.Name,
			shared.PayloadKeyNamespace: item.Namespace,
			shared.PayloadKeyManifest:  item.Manifest,
		})
	}
	return map[string]any{
		shared.PayloadKeyResources: resourcePayload,
		shared.PayloadKeyNote:      constants.SnapshotNote,
	}
}

func NewSnapshotID() (string, error) {
	u := uuid.New()
	hexStr := hex.EncodeToString(u[:])
	if len(hexStr) < snapshotIDSuffixLen {
		return constants.EmptyString, errors.New("uuid hex too short")
	}
	return snapshotIDPrefix + hexStr[:snapshotIDSuffixLen], nil
}

func PayloadFromUnstructured(objs []unstructured.Unstructured) map[string]any {
	if len(objs) == constants.DefaultInitValue {
		return map[string]any{
			shared.PayloadKeyResources: []map[string]any{},
			shared.PayloadKeyNote:      constants.SnapshotNote,
		}
	}
	slices.SortFunc(objs, func(a, b unstructured.Unstructured) int {
		return cmp.Compare(kindOrder(a.GetKind()), kindOrder(b.GetKind()))
	})

	resourcePayload := make([]map[string]any, constants.DefaultInitValue, len(objs))
	for i := range objs {
		obj := objs[i].DeepCopy()
		if obj == nil || obj.Object == nil {
			continue
		}
		kcoremanifest.CleanManifestForApply(obj.Object)
		resourcePayload = append(resourcePayload, map[string]any{
			shared.PayloadKeyKind:      obj.GetKind(),
			shared.PayloadKeyName:      obj.GetName(),
			shared.PayloadKeyNamespace: obj.GetNamespace(),
			shared.PayloadKeyManifest:  obj.Object,
		})
	}
	return map[string]any{
		shared.PayloadKeyResources: resourcePayload,
		shared.PayloadKeyNote:      constants.SnapshotNote,
	}
}
