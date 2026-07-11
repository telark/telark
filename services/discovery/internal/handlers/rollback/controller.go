package rollback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/clients"
	dconfig "github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/helpers/async"
	notifclient "github.com/telark/rest/clients/notifications"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	kcorefactory "github.com/telark/kcore/informers/factory"
	kcorek8s "github.com/telark/kcore/k8sclient"
	kcoreapply "github.com/telark/kcore/ops/apply"
)

const (
	hashPrefixLen       = 8
	unknownKindRank     = 99
	invalidIndex        = -1
	splitPathPartsLimit = 2
	notifEmitTimeout    = 5 * time.Second
)

var crdGVR = schema.GroupVersionResource{
	Group:    "erpi.telark",
	Version:  "v1alpha1",
	Resource: "applicationsasresources",
}

var kindRank = map[string]int{
	constants.RollbackKindServiceAccount: 1,
	constants.RollbackKindSecret:         2,
	constants.RollbackKindConfigMap:      3,
	constants.RollbackKindService:        4,
	constants.RollbackKindNetworkPolicy:  5,
	constants.RollbackKindDeployment:     6,
	constants.RollbackKindStatefulSet:    7,
	constants.RollbackKindDaemonSet:      8,
	constants.RollbackKindJob:            9,
	constants.RollbackKindCronJob:        10,
}

type Controller struct {
	kubeClient     *kubernetes.Clientset
	dyn            dynamic.Interface
	mapper         meta.RESTMapper
	queue          workqueue.TypedRateLimitingInterface[string]
	snapshotClient *clients.SnapshotClient
	notifClient    *clients.NotificationClient
}

type rollbackPatchOpts struct {
	Status      string
	ErrorMsg    string
	CompletedAt *time.Time
}

func NewController(kubeClient *kubernetes.Clientset) *Controller {
	return &Controller{
		kubeClient:     kubeClient,
		queue:          workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		snapshotClient: clients.NewSnapshotClient(),
		notifClient:    clients.NewNotificationClient(),
	}
}

func (c *Controller) Run(ctx context.Context) {
	if err := c.initClients(); err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrFailedInitRollbackController), err))
		return
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		c.dyn,
		kcorefactory.JitteredResync(dconfig.RollbackInformerResync(), dconfig.InformerResyncJitterFraction()),
		metav1.NamespaceAll,
		nil,
	)
	inf := factory.ForResource(crdGVR).Informer()

	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueue,
		UpdateFunc: func(_, newObj any) { c.enqueue(newObj) },
	})

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		logger.Error(string(constants.ErrRollbackControllerCacheSyncFailed))
		return
	}

	go c.worker(ctx)
	<-ctx.Done()
}

func (c *Controller) initClients() error {
	dyn, err := kcorek8s.InitDynamicClient()
	if err != nil {
		return err
	}
	c.dyn = dyn
	c.mapper = kcorek8s.NewDeferredRESTMapper(c.kubeClient)
	return nil
}

func (c *Controller) enqueue(obj any) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	key := fmt.Sprintf("%s/%s", u.GetNamespace(), u.GetName())
	c.queue.Add(key)
}

func (c *Controller) worker(ctx context.Context) {
	for {
		item, shutdown := c.queue.Get()
		if shutdown {
			return
		}
		func() {
			defer c.queue.Done(item)
			if err := c.reconcile(ctx, item); err != nil {
				if isTransientBackpressure(err) {
					logger.Debug(fmt.Sprintf(string(constants.LogRollbackReconcileBackpressure), item))
					c.queue.AddRateLimited(item)
					return
				}
				logger.Error(fmt.Sprintf(string(constants.ErrRollbackReconcileFailed), item, err))
				c.queue.AddRateLimited(item)
				return
			}
			c.queue.Forget(item)
		}()
	}
}

func isTransientBackpressure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, constants.ClientRateLimiterWaitErrorSubstr)
}

func (c *Controller) reconcile(ctx context.Context, key string) error {
	ns, name, ok := splitKey(key)
	if !ok {
		return nil
	}

	cr, err := c.dyn.Resource(crdGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	spec, err := extractSpec(cr)
	if err != nil {
		return err
	}

	if handled, err := c.failStaleInProgress(ctx, ns, name, spec); err != nil {
		return err
	} else if handled {
		return nil
	}

	pending, _ := findPending(spec.Rollbacks)
	if pending == nil {
		return nil
	}

	logger.Info(fmt.Sprintf(string(constants.InfoPickedUpPendingEntry), pending.ID))
	return c.processPending(ctx, ns, name, pending)
}

func (c *Controller) processPending(
	ctx context.Context,
	ns, name string,
	pending *application.RollbackEntry,
) error {
	procCtx, cancel := context.WithTimeout(ctx, constants.RollbackProcessTimeout)
	defer cancel()

	logger.Info(string(constants.InfoPatchingStatusToInProgress))
	spec, idx, err := c.refetchAndVerifyPending(procCtx, ns, name, pending.ID)
	if err != nil {
		return err
	}
	if spec == nil {
		logger.Info(fmt.Sprintf(string(constants.InfoRollbackNoLongerPending), pending.ID))
		return nil
	}
	if err := c.patchRollbackStatus(procCtx, ns, name, spec, idx, rollbackPatchOpts{
		Status: constants.RollbackStatusInProgress,
	}); err != nil {
		return fmt.Errorf(string(constants.ErrRollbackStatusPatchFailed), err)
	}

	targetSnapshots := snapshotsByGeneration(spec.Snapshots, pending.TargetGeneration)
	if len(targetSnapshots) == constants.DefaultInitValue {
		c.failRollback(procCtx, ns, name, spec, idx, fmt.Sprintf(
			string(constants.ErrRollbackSnapshotFetchFailed),
			errors.New("snapshot not found"),
		))
		return nil
	}
	manifest, err := c.loadRollbackManifest(procCtx, targetSnapshots)
	if err != nil {
		c.failRollback(procCtx, ns, name, spec, idx, fmt.Sprintf(string(constants.ErrRollbackSnapshotFetchFailed), err))
		return nil
	}

	if err := c.validateAndApplyRollback(procCtx, manifest, ns, name, spec, idx); err != nil {
		return nil
	}
	return c.finalizeRollbackSuccess(procCtx, ns, name, spec, pending, idx)
}

func (c *Controller) validateAndApplyRollback(
	ctx context.Context,
	manifest []unstructured.Unstructured,
	ns, name string,
	spec *application.Application,
	idx int,
) error {
	sorted := sortManifestForApply(manifest)
	if err := c.validateRollbackManifest(ctx, sorted); err != nil {
		c.failRollback(ctx, ns, name, spec, idx, err.Error())
		return err
	}
	if err := c.applyAllWithRetry(ctx, sorted); err != nil {
		c.failRollback(ctx, ns, name, spec, idx, fmt.Sprintf(string(constants.ErrRollbackApplyFailed), err))
		return err
	}
	return nil
}

func (c *Controller) finalizeRollbackSuccess(
	ctx context.Context,
	ns, name string,
	spec *application.Application,
	pending *application.RollbackEntry,
	idx int,
) error {
	if err := c.appendHistoryChangeLog(ctx, ns, name, spec, pending); err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrRollbackHistoryAppendFailed), err))
	}

	now := time.Now().UTC()
	if err := c.patchRollbackStatus(ctx, ns, name, spec, idx, rollbackPatchOpts{
		Status:      constants.RollbackStatusSuccess,
		CompletedAt: &now,
	}); err != nil {
		return err
	}

	c.emitRollbackSuccess(pending, name)
	return nil
}

func (c *Controller) emitRollbackSuccess(pending *application.RollbackEntry, appName string) {
	if pending == nil || pending.TriggeredBy == "" {
		return
	}
	message := fmt.Sprintf(string(constants.NotifRollbackCompletedFormat), appName, pending.TargetGeneration)
	c.dispatchRollbackNotif(pending, appName, string(constants.NotifRollbackCompletedTitle), message,
		notifclient.SeveritySuccess, notifclient.RollbackStatusSuccess)
}

func (c *Controller) emitRollbackFailure(pending *application.RollbackEntry, appName, errMsg string) {
	if pending == nil || pending.TriggeredBy == "" {
		return
	}
	message := fmt.Sprintf(string(constants.NotifRollbackFailedFormat), appName, pending.TargetGeneration, errMsg)
	c.dispatchRollbackNotif(pending, appName, string(constants.NotifRollbackFailedTitle), message,
		notifclient.SeverityError, notifclient.RollbackStatusFailure)
}

func (c *Controller) dispatchRollbackNotif(
	pending *application.RollbackEntry,
	appName, title, message, severity, status string,
) {
	n := notifclient.Notification{
		UserID:   pending.TriggeredBy,
		Type:     notifclient.TypeRollbackCompleted,
		Title:    title,
		Message:  message,
		Severity: severity,
		Metadata: map[string]any{
			notifclient.MetaKeyTargetID:        pending.ID,
			notifclient.MetaKeyApplicationID:   pending.ID,
			notifclient.MetaKeyApplicationName: appName,
			notifclient.MetaKeyStatus:          status,
		},
	}
	notifier := c.notifClient
	async.Dispatch(func(ctx context.Context) {
		_ = notifier.Emit(ctx, n)
	})
}

func (c *Controller) failRollback(
	ctx context.Context,
	ns, name string,
	spec *application.Application,
	idx int,
	message string,
) {
	_ = c.patchRollbackStatus(ctx, ns, name, spec, idx, rollbackPatchOpts{
		Status:   constants.RollbackStatusFailed,
		ErrorMsg: message,
	})
	if idx >= constants.DefaultInitValue && idx < len(spec.Rollbacks) {
		c.emitRollbackFailure(&spec.Rollbacks[idx], name, message)
	}
}

func snapshotsByGeneration(
	snapshots []application.ApplicationSnapshot,
	generation int,
) []application.ApplicationSnapshot {
	out := make([]application.ApplicationSnapshot, constants.DefaultInitValue)
	for i := range snapshots {
		if snapshots[i].Generation == generation {
			out = append(out, snapshots[i])
		}
	}
	return out
}

func (c *Controller) loadRollbackManifest(
	ctx context.Context,
	targetSnapshots []application.ApplicationSnapshot,
) ([]unstructured.Unstructured, error) {
	out := make([]unstructured.Unstructured, constants.DefaultInitValue)
	for i := range targetSnapshots {
		scope := scopeFromSnapshotPath(targetSnapshots[i].Path)
		manifest, err := c.getSnapshotManifestWithRetry(
			ctx,
			targetSnapshots[i].ID,
			scope,
			targetSnapshots[i].Namespace,
			targetSnapshots[i].Generation,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, manifest...)
	}
	return out, nil
}

func (c *Controller) failStaleInProgress(
	ctx context.Context,
	ns, name string,
	spec *application.Application,
) (bool, error) {
	for i := range spec.Rollbacks {
		rb := spec.Rollbacks[i]
		if rb.Status != constants.RollbackStatusInProgress {
			continue
		}
		if time.Since(rb.TriggeredAt) <= constants.RollbackStaleInProgressAfter {
			continue
		}
		now := time.Now().UTC()
		if err := c.patchRollbackStatus(ctx, ns, name, spec, i, rollbackPatchOpts{
			Status:      constants.RollbackStatusFailed,
			ErrorMsg:    string(constants.ErrRollbackInterruptedRestart),
			CompletedAt: &now,
		}); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func (c *Controller) refetchAndVerifyPending(
	ctx context.Context,
	ns, name, rollbackID string,
) (*application.Application, int, error) {
	cr, err := c.dyn.Resource(crdGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, invalidIndex, err
	}
	spec, err := extractSpec(cr)
	if err != nil {
		return nil, invalidIndex, err
	}
	for i := range spec.Rollbacks {
		if spec.Rollbacks[i].ID != rollbackID {
			continue
		}
		if spec.Rollbacks[i].Status != constants.RollbackStatusPending {
			return nil, invalidIndex, nil
		}
		return spec, i, nil
	}
	return nil, invalidIndex, nil
}

func (c *Controller) patchRollbackStatus(
	ctx context.Context,
	ns, name string,
	spec *application.Application,
	idx int,
	opts rollbackPatchOpts,
) error {
	updated := slices.Clone(spec.Rollbacks)
	if idx < constants.DefaultInitValue || idx >= len(updated) {
		return errors.New(string(constants.ErrRollbackInvalidIndex))
	}
	updated[idx].Status = opts.Status
	if opts.ErrorMsg != constants.EmptyString {
		updated[idx].Error = opts.ErrorMsg
	}
	if opts.CompletedAt != nil {
		updated[idx].CompletedAt = opts.CompletedAt
	}

	patch := map[string]any{
		constants.RollbackSpecKey: map[string]any{
			constants.RollbackRollbacksKey: updated,
		},
	}
	data, err := jsonBytes(patch)
	if err != nil {
		return fmt.Errorf(string(constants.ErrRollbackMarshalPatchFailed), err)
	}
	patchCtx, cancel := context.WithTimeout(ctx, constants.RollbackPatchTimeout)
	defer cancel()
	_, err = c.dyn.Resource(crdGVR).
		Namespace(ns).
		Patch(patchCtx, name, types.MergePatchType, data, metav1.PatchOptions{})
	return err
}

func (c *Controller) applyAll(ctx context.Context, resources []unstructured.Unstructured) error {
	return kcoreapply.ApplyUnstructuredServerSide(
		ctx,
		c.dyn,
		c.mapper,
		resources,
		kcoreapply.ServerSideApplyOptions{
			FieldManager: constants.RollbackFieldManager,
			Force:        true,
			DryRun:       false,
		},
		func(kind, name, namespace string) {
			logger.Info(fmt.Sprintf(string(constants.InfoRollbackApplied), kind, name, namespace))
		},
	)
}

func (c *Controller) dryRunApplyAll(ctx context.Context, resources []unstructured.Unstructured) error {
	return kcoreapply.ApplyUnstructuredServerSide(
		ctx,
		c.dyn,
		c.mapper,
		resources,
		kcoreapply.ServerSideApplyOptions{
			FieldManager: constants.RollbackFieldManager,
			Force:        true,
			DryRun:       true,
		},
		nil,
	)
}

func (c *Controller) validateRollbackManifest(ctx context.Context, resources []unstructured.Unstructured) error {
	if err := c.ensureNamespacesExist(ctx, resources); err != nil {
		return err
	}
	if err := c.ensureAPIVersionsServed(resources); err != nil {
		return err
	}
	if err := c.dryRunApplyAll(ctx, resources); err != nil {
		return fmt.Errorf(string(constants.ErrRollbackDryRunFailed), err)
	}
	return nil
}

func (c *Controller) ensureNamespacesExist(ctx context.Context, resources []unstructured.Unstructured) error {
	checked := make(map[string]struct{}, len(resources))
	for i := range resources {
		ns := strings.TrimSpace(resources[i].GetNamespace())
		if ns == constants.EmptyString {
			continue
		}
		if _, ok := checked[ns]; ok {
			continue
		}
		if _, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); err != nil {
			return fmt.Errorf(string(constants.ErrRollbackNamespaceMissing), ns)
		}
		checked[ns] = struct{}{}
	}
	return nil
}

func (c *Controller) ensureAPIVersionsServed(resources []unstructured.Unstructured) error {
	for i := range resources {
		gvk := resources[i].GroupVersionKind()
		if _, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
			return fmt.Errorf(string(constants.ErrRollbackAPIVersionNotServed), gvk.GroupVersion().String(), gvk.Kind)
		}
	}
	return nil
}

func (c *Controller) applyAllWithRetry(ctx context.Context, resources []unstructured.Unstructured) error {
	var lastErr error
	for attempt := constants.DefaultAddValue; attempt <= constants.RollbackRetryMaxAttempts; attempt++ {
		applyCtx, cancel := context.WithTimeout(ctx, constants.RollbackApplyTimeout)
		err := c.applyAll(applyCtx, resources)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < constants.RollbackRetryMaxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(constants.RollbackRetryInterval):
			}
		}
	}
	return lastErr
}

func (c *Controller) getSnapshotManifestWithRetry(
	ctx context.Context,
	snapshotID string,
	scope string,
	namespace string,
	generation int,
) ([]unstructured.Unstructured, error) {
	var lastErr error
	for attempt := constants.DefaultAddValue; attempt <= constants.RollbackRetryMaxAttempts; attempt++ {
		fetchCtx, cancel := context.WithTimeout(ctx, constants.RollbackSnapshotFetchTimeout)
		out, err := c.snapshotClient.GetSnapshotManifest(fetchCtx, snapshotID, scope, namespace, generation)
		cancel()
		if err == nil {
			return out, nil
		}
		lastErr = err
		if attempt < constants.RollbackRetryMaxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(constants.RollbackRetryInterval):
			}
		}
	}
	return nil, lastErr
}

func (c *Controller) appendHistoryChangeLog(
	ctx context.Context,
	ns, name string,
	spec *application.Application,
	entry *application.RollbackEntry,
) error {
	gen := nextChangeLogGeneration(spec.History.ChangeLog, spec.History.Generation)
	now := time.Now().UTC().Format(time.RFC3339)
	fingerprint := shortHash(entry.ID)
	newValue := entry.TargetSnapshotID

	ch := application.ChangeLogEntry{
		Generation:  gen,
		DetectedAt:  now,
		ChangeClass: constants.RollbackChangeClass,
		Severity:    constants.RollbackSeverityLow,
		ChangedBy:   entry.TriggeredBy,
		Fingerprint: fingerprint,
		IsIncident:  false,
		IsRecovery:  true,
		Changes: []application.ApplicationChange{{
			ChangeType: constants.RollbackChangeType,
			Description: fmt.Sprintf(
				constants.RollbackHistoryDescriptionFormat,
				entry.TargetGeneration,
				entry.TargetSnapshotID,
			),
			Field:    constants.RollbackChangeField,
			NewValue: &newValue,
		}},
	}

	updated := append(slices.Clone(spec.History.ChangeLog), ch)
	patch := map[string]any{
		constants.RollbackSpecKey: map[string]any{
			constants.RollbackHistoryKey: map[string]any{
				constants.RollbackChangeLogKey: updated,
			},
		},
	}
	data, err := jsonBytes(patch)
	if err != nil {
		return fmt.Errorf(string(constants.ErrRollbackMarshalPatchFailed), err)
	}
	patchCtx, cancel := context.WithTimeout(ctx, constants.RollbackPatchTimeout)
	defer cancel()
	_, err = c.dyn.Resource(crdGVR).
		Namespace(ns).
		Patch(patchCtx, name, types.MergePatchType, data, metav1.PatchOptions{})
	return err
}

func extractSpec(cr *unstructured.Unstructured) (*application.Application, error) {
	specMap, ok := cr.Object[constants.RollbackSpecKey].(map[string]any)
	if !ok {
		return nil, errors.New(string(constants.ErrRollbackMissingSpec))
	}
	var spec application.Application
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func sortManifestForApply(in []unstructured.Unstructured) []unstructured.Unstructured {
	out := slices.Clone(in)
	slices.SortStableFunc(out, func(a, b unstructured.Unstructured) int {
		return kindOrder(a.GetKind()) - kindOrder(b.GetKind())
	})
	return out
}

func kindOrder(kind string) int {
	if rank, ok := kindRank[kind]; ok {
		return rank
	}
	return unknownKindRank
}

func findPending(rollbacks []application.RollbackEntry) (*application.RollbackEntry, int) {
	for i := range rollbacks {
		if rollbacks[i].Status == constants.RollbackStatusPending {
			return &rollbacks[i], i
		}
	}
	return nil, invalidIndex
}

func splitKey(key string) (ns string, name string, ok bool) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+constants.DefaultAddValue:], true
		}
	}
	return constants.EmptyString, constants.EmptyString, false
}

func nextChangeLogGeneration(list []application.ChangeLogEntry, current int) int {
	maxGen := current
	for i := range list {
		if list[i].Generation > maxGen {
			maxGen = list[i].Generation
		}
	}
	return maxGen + constants.DefaultAddValue
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:hashPrefixLen]
}

func jsonBytes(v any) ([]byte, error) {
	return json.Marshal(v)
}

func scopeFromSnapshotPath(path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, constants.RollbackSnapshotPathScopePrefix) {
		return constants.RollbackSnapshotPathDefaultScope
	}
	rest := strings.TrimPrefix(path, constants.RollbackSnapshotPathScopePrefix)
	if rest == constants.EmptyString {
		return constants.RollbackSnapshotPathDefaultScope
	}
	parts := strings.SplitN(rest, "/", splitPathPartsLimit)
	if strings.TrimSpace(parts[constants.DefaultInitValue]) == constants.EmptyString {
		return constants.RollbackSnapshotPathDefaultScope
	}
	return strings.TrimSpace(parts[constants.DefaultInitValue])
}
