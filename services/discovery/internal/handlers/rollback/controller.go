package rollback

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/clients"
	dconfig "github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/helpers/async"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	notifclient "github.com/telark/rest/clients/notifications"
	xwareredis "github.com/telark/x-ware/redis/stream"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	kcoreapi "github.com/telark/kcore/crds/api"
	crdview "github.com/telark/kcore/crds/view"
	kcorefactory "github.com/telark/kcore/informers/factory"
	kcorek8s "github.com/telark/kcore/k8sclient"
	kcoreshared "github.com/telark/kcore/shared"
)

const (
	unknownKindRank     = 99
	invalidIndex        = -1
	splitPathPartsLimit = 2
	notifEmitTimeout    = 5 * time.Second
)

var errRollbackLockBusy = errors.New(string(constants.ErrRollbackLockBusy))

var crdGVR = schema.GroupVersionResource{
	Group:    v1alpha1.ApplicationMetadata.BaseGroup,
	Version:  v1alpha1.ApplicationMetadata.Version,
	Resource: v1alpha1.ApplicationMetadata.Plural,
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

func NewController(kubeClient *kubernetes.Clientset) *Controller {
	return &Controller{
		kubeClient:     kubeClient,
		snapshotClient: clients.NewSnapshotClient(),
		notifClient:    clients.NewNotificationClient(),
	}
}

func (c *Controller) Run(ctx context.Context) {
	if err := c.initClients(); err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrFailedInitRollbackController), err))
		return
	}

	// Per Run: leadergate re-runs the same Controller each leadership term and
	// a shut-down workqueue never accepts items again.
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		c.dyn,
		kcorefactory.JitteredResync(dconfig.RollbackInformerResync(), dconfig.InformerResyncJitterFraction()),
		metav1.NamespaceAll,
		nil,
	)
	inf := factory.ForResource(crdGVR).Informer()

	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { enqueue(queue, obj) },
		UpdateFunc: func(_, newObj any) { enqueue(queue, newObj) },
	})

	factory.Start(ctx.Done())
	defer factory.Shutdown()
	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		logger.Error(string(constants.ErrRollbackControllerCacheSyncFailed))
		return
	}

	workers := dconfig.RollbackWorkers()
	logger.Info(fmt.Sprintf(string(constants.InfoRollbackWorkersStarted), workers))
	RunWorkers(ctx, queue, workers, c.reconcile)
}

func (c *Controller) initClients() error {
	qps, burst := dconfig.RollbackK8sClientRateLimit()
	dyn, err := kcorek8s.NewDynamicClientWithRateLimit(qps, burst)
	if err != nil {
		return err
	}
	c.dyn = dyn
	logger.Info(fmt.Sprintf(string(constants.InfoRollbackClientBudget), qps, burst))
	c.mapper = kcorek8s.NewDeferredRESTMapper(c.kubeClient)
	return nil
}

func enqueue(queue workqueue.TypedRateLimitingInterface[string], obj any) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	key := fmt.Sprintf("%s/%s", u.GetNamespace(), u.GetName())
	queue.Add(key)
}

// Workers only ever overlap on different apps: the workqueue hands a key to one
// worker at a time and claimPending holds the per-app Redis lock.
func RunWorkers(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[string],
	workers int,
	reconcile func(context.Context, string) error,
) {
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() { worker(ctx, queue, reconcile) })
	}
	<-ctx.Done()
	queue.ShutDown()
	wg.Wait()
}

func worker(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[string],
	reconcile func(context.Context, string) error,
) {
	for {
		item, shutdown := queue.Get()
		if shutdown {
			return
		}
		func() {
			defer queue.Done(item)
			if err := reconcile(ctx, item); err != nil {
				if isTransientBackpressure(err) {
					logger.Debug(fmt.Sprintf(string(constants.LogRollbackReconcileBackpressure), item))
					queue.AddRateLimited(item)
					return
				}
				logger.Error(fmt.Sprintf(string(constants.ErrRollbackReconcileFailed), item, err))
				queue.AddRateLimited(item)
				return
			}
			queue.Forget(item)
		}()
	}
}

func isTransientBackpressure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errRollbackLockBusy) {
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

	if handled, err := c.FailStaleInProgress(ctx, name, spec); err != nil {
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
	spec, idx, err := c.claimPending(procCtx, ns, name, pending.ID)
	if err != nil {
		return err
	}
	if spec == nil {
		logger.Info(fmt.Sprintf(string(constants.InfoRollbackNoLongerPending), pending.ID))
		return nil
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

	if err := c.validateAndApplyRollback(procCtx, manifest, ns, name, spec, idx, pending); err != nil {
		return nil
	}
	return c.FinalizeRollbackSuccess(procCtx, name, spec, pending, idx)
}

// Holds the key the trigger/abort handlers take, so an abort cannot land
// between the pending check and the whole-array in_progress patch that would
// otherwise overwrite it.
func (c *Controller) claimPending(
	ctx context.Context,
	ns, name, rollbackID string,
) (*application.Application, int, error) {
	release, err := lockRollback(ctx, name)
	if err != nil {
		return nil, invalidIndex, err
	}
	defer release()
	spec, idx, err := c.refetchAndVerifyPending(ctx, ns, name, rollbackID)
	if err != nil || spec == nil {
		return nil, invalidIndex, err
	}
	if err := patchRollbackStatus(ctx, name, spec, idx, rollbackPatchOpts{
		Status: constants.RollbackStatusInProgress,
	}); err != nil {
		return nil, invalidIndex, fmt.Errorf(string(constants.ErrRollbackStatusPatchFailed), err)
	}
	return spec, idx, nil
}

// A busy lock is a handler mid-request; the sentinel is requeued as
// backpressure so pickup retries within milliseconds instead of at resync.
func lockRollback(ctx context.Context, name string) (func(), error) {
	rdb := redishelper.NewRedisClient()
	if rdb == nil {
		return func() {}, nil
	}
	lock := xwareredis.NewLockClient(rdb)
	key := constants.KeyPrefixLockRollback + name
	value := uuid.NewString()
	acquired, err := lock.Acquire(ctx, key, value, constants.DefaultLockTTL)
	if err != nil {
		return nil, err
	}
	if !acquired {
		logger.Debug(fmt.Sprintf(string(constants.LogRollbackLockBusy), name))
		return nil, errRollbackLockBusy
	}
	return func() { _ = lock.Release(context.Background(), key, value) }, nil
}

func (c *Controller) validateAndApplyRollback(
	ctx context.Context,
	manifest []unstructured.Unstructured,
	ns, name string,
	spec *application.Application,
	idx int,
	pending *application.RollbackEntry,
) error {
	sorted := withoutJobRuns(sortManifestForApply(manifest), name)
	if err := c.validateRollbackManifest(ctx, sorted); err != nil {
		c.failRollback(ctx, ns, name, spec, idx, err.Error())
		return err
	}
	markRollbackApplying(ctx, name, pending)
	if err := c.applyAllWithRetry(ctx, sorted); err != nil {
		c.failRollback(ctx, ns, name, spec, idx, fmt.Sprintf(string(constants.ErrRollbackApplyFailed), err))
		return err
	}
	return nil
}

func (c *Controller) FinalizeRollbackSuccess(
	ctx context.Context,
	name string,
	spec *application.Application,
	pending *application.RollbackEntry,
	idx int,
) error {
	if err := appendHistoryChangeLog(ctx, name, spec, pending); err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrRollbackHistoryAppendFailed), err))
	}

	now := time.Now().UTC()
	restored := pending.TargetGeneration
	if err := patchRollbackStatus(ctx, name, spec, idx, rollbackPatchOpts{
		Status:             constants.RollbackStatusSuccess,
		CompletedAt:        &now,
		RestoredGeneration: &restored,
	}); err != nil {
		return err
	}

	c.emitRollbackSuccess(pending, name)
	return nil
}

func (c *Controller) emitRollbackSuccess(pending *application.RollbackEntry, appName string) {
	if pending == nil || pending.TriggeredBy == constants.EmptyString {
		return
	}
	message := fmt.Sprintf(string(constants.NotifRollbackCompletedFormat), appName, pending.TargetGeneration)
	c.dispatchRollbackNotif(pending, appName, string(constants.NotifRollbackCompletedTitle), message,
		notifclient.SeveritySuccess, notifclient.RollbackStatusSuccess)
}

func (c *Controller) emitRollbackFailure(pending *application.RollbackEntry, appName, errMsg string) {
	if pending == nil || pending.TriggeredBy == constants.EmptyString {
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
			notifclient.MetaKeyApplicationID:   appName,
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
	now := time.Now().UTC()
	opts := rollbackPatchOpts{
		Status:      constants.RollbackStatusFailed,
		ErrorMsg:    message,
		CompletedAt: &now,
	}
	err := RecordWithRetry(ctx, func(recordCtx context.Context) error {
		return patchRollbackStatus(recordCtx, name, spec, idx, opts)
	})
	if err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrRollbackFailureRecordFailed), ns, name, message, err))
	}
	if idx >= constants.DefaultInitValue && idx < len(spec.Rollbacks) {
		c.emitRollbackFailure(&spec.Rollbacks[idx], name, message)
	}
}

// Detached from ctx's cancellation: a blown process deadline is itself a common reason a
// rollback failed, and the failure still has to reach the CR. Bounded by attempts and one timeout.
func RecordWithRetry(ctx context.Context, record func(context.Context) error) error {
	recordCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		constants.RollbackFailureRecordTimeout,
	)
	defer cancel()

	var lastErr error
	for attempt := constants.DefaultAddValue; attempt <= constants.RollbackRetryMaxAttempts; attempt++ {
		err := record(recordCtx)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < constants.RollbackRetryMaxAttempts {
			select {
			case <-recordCtx.Done():
				return lastErr
			case <-time.After(constants.RollbackRetryInterval):
			}
		}
	}
	return lastErr
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

func (c *Controller) FailStaleInProgress(
	ctx context.Context,
	name string,
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
		msg := StaleSweepErrorMsg(rb.Error)
		if err := patchRollbackStatus(ctx, name, spec, i, rollbackPatchOpts{
			Status:      constants.RollbackStatusFailed,
			ErrorMsg:    msg,
			CompletedAt: &now,
		}); err != nil {
			return true, err
		}
		c.emitRollbackFailure(&spec.Rollbacks[i], name, cmp.Or(msg, rb.Error))
		return true, nil
	}
	return false, nil
}

// Empty (leave the stored error untouched) when a reason is already recorded: the sweep is a
// last resort, and the generic restart text would destroy the only copy of a real failure.
func StaleSweepErrorMsg(recorded string) string {
	if strings.TrimSpace(recorded) != constants.EmptyString {
		return constants.EmptyString
	}
	return string(constants.ErrRollbackInterruptedRestart)
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

func patchRollbackStatus(
	ctx context.Context,
	name string,
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
	if opts.RestoredGeneration != nil {
		updated[idx].RestoredGeneration = opts.RestoredGeneration
	}

	return patchStatus(ctx, name, map[string]any{constants.RollbackRollbacksKey: updated})
}

// Rollbacks and history are observed state: they live under .status, which a
// main-resource patch would silently drop.
func patchStatus(ctx context.Context, name string, status map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	res := kcoreapi.PatchCustomResourceStatus(v1alpha1.ApplicationMetadata, name,
		map[string]any{constants.RollbackStatusKey: status})
	if res.Status != kcoreshared.StatusOK {
		return fmt.Errorf(string(constants.ErrRollbackStatusWriteFailed), res.Status, res.Error)
	}
	return nil
}

func (c *Controller) applyAll(ctx context.Context, resources []unstructured.Unstructured) error {
	return ReplaceUnstructured(ctx, c.dyn, c.mapper, resources, false, func(kind, name, namespace string) {
		logger.Info(fmt.Sprintf(string(constants.InfoRollbackApplied), kind, name, namespace))
	})
}

func (c *Controller) dryRunApplyAll(ctx context.Context, resources []unstructured.Unstructured) error {
	return ReplaceUnstructured(ctx, c.dyn, c.mapper, resources, true, nil)
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

func appendHistoryChangeLog(
	ctx context.Context,
	name string,
	spec *application.Application,
	entry *application.RollbackEntry,
) error {
	gen := nextChangeLogGeneration(spec.History.ChangeLog, spec.History.Generation)
	now := time.Now().UTC().Format(time.RFC3339)
	fingerprint := diff.RollbackFingerprint(entry.ID)
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
	return patchStatus(ctx, name, map[string]any{
		constants.RollbackHistoryKey: map[string]any{
			constants.RollbackChangeLogKey: updated,
			// The diff derives its next generation from this field alone, so leaving it
			// behind makes the next diff-authored entry reuse gen.
			constants.RollbackGenerationKey: gen,
		},
	})
}

func extractSpec(cr *unstructured.Unstructured) (*application.Application, error) {
	view := crdview.ToView(cr, v1alpha1.ApplicationMetadata)
	if view == nil {
		return nil, errors.New(string(constants.ErrRollbackMissingSpec))
	}
	var spec application.Application
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(view, &spec); err != nil {
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
	ns, name, ok = strings.Cut(key, constants.PathSeparator)
	if !ok {
		return constants.EmptyString, constants.EmptyString, false
	}
	return ns, name, true
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

func markRollbackApplying(ctx context.Context, appName string, pending *application.RollbackEntry) {
	rdb := redishelper.NewRedisClient()
	if rdb == nil {
		return
	}
	raw, err := json.Marshal(diff.RollbackMarker{
		ID:               pending.ID,
		TargetGeneration: pending.TargetGeneration,
		TargetSnapshotID: pending.TargetSnapshotID,
		TriggeredBy:      pending.TriggeredBy,
	})
	if err != nil {
		return
	}
	_ = rdb.Set(ctx, constants.KeyPrefixRollbackApplying+appName, raw, constants.RollbackApplyingTTL).Err()
}

// withoutJobRuns drops Job manifests: re-applying a finished or deleted Job
// would start a new run, which a rollback must never do.
func withoutJobRuns(in []unstructured.Unstructured, appName string) []unstructured.Unstructured {
	out := make([]unstructured.Unstructured, constants.DefaultInitValue, len(in))
	for i := range in {
		if in[i].GetKind() == constants.RollbackKindJob {
			logger.Info(fmt.Sprintf(string(constants.InfoRollbackSkippedJob), appName, in[i].GetNamespace(), in[i].GetName()))
			continue
		}
		out = append(out, in[i])
	}
	return out
}
