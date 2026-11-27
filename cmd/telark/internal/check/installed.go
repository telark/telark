package check

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/version"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

var expectedCRDs = []string{
	v1alpha1.PluralApplications, v1alpha1.PluralProtectionPlans, v1alpha1.PluralTelarkConfigs,
	v1alpha1.PluralCategories, v1alpha1.PluralUsers, v1alpha1.PluralGroups,
	v1alpha1.PluralAccessRoles, v1alpha1.PluralPasskeys, v1alpha1.PluralSessions,
	v1alpha1.PluralAuthRequests,
}

func (r *runner) installed(ctx context.Context) []Result {
	telark, release, err := r.deployments(ctx)
	results := []Result{r.crds(ctx)}
	results = append(results, r.release(ctx, telark, release, err)...)
	results = append(results, r.kyverno(ctx), r.crdGuard(ctx))
	results = append(results, r.bootstrap(ctx, telark)...)
	return append(results, r.plans(ctx))
}

// One list serves both: the Telark services carry the release label, and the subcharts' Deployments share it.
func (r *runner) deployments(ctx context.Context) (telark, release []appsv1.Deployment, err error) {
	list, err := r.core.AppsV1().Deployments(r.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf(constants.ErrListDeployments, r.namespace, err)
	}
	for i := range list.Items {
		if list.Items[i].Labels[constants.LabelPartOf] == constants.AppName {
			telark = append(telark, list.Items[i])
		}
	}
	if len(telark) == constants.DefaultInitValue {
		return nil, nil, fmt.Errorf(constants.ErrNoDeployments, r.namespace)
	}
	for i := range list.Items {
		if list.Items[i].Labels[constants.LabelInstance] == instanceOf(telark) {
			release = append(release, list.Items[i])
		}
	}
	return telark, release, nil
}

func instanceOf(telark []appsv1.Deployment) string {
	return telark[constants.DefaultInitValue].Labels[constants.LabelInstance]
}

// Without the Telark Deployments there is no release to check, so one failure stands for all three checks.
func (r *runner) release(ctx context.Context, telark, release []appsv1.Deployment, err error) []Result {
	if err != nil {
		return []Result{ReadFailed(constants.CheckWorkloads, constants.ReadWorkloads, err)}
	}
	selector := metav1.ListOptions{LabelSelector: constants.LabelInstance + constants.SelectorEquals + instanceOf(telark)}
	return []Result{r.workloads(ctx, release, selector), r.restarts(ctx, selector), versionMatch(r.namespace, telark)}
}

func (r *runner) crds(ctx context.Context) Result {
	found, err := r.telarkCRDs(ctx)
	if err != nil {
		return ReadFailed(constants.CheckCRDs, constants.ReadCRDs, err)
	}
	var missing, versions []string
	for _, plural := range expectedCRDs {
		name := qualified(plural, base.Group)
		crd, ok := found[name]
		if !ok || !established(crd) {
			missing = append(missing, name)
			continue
		}
		versions = append(versions, cmp.Or(crd.Labels[constants.LabelVersion], constants.UnknownValue))
	}
	if len(missing) > constants.DefaultInitValue {
		return Fail(constants.CheckCRDs, fmt.Sprintf(constants.MsgCRDsMissing, strings.Join(missing, constants.ListSeparator)))
	}
	slices.Sort(versions)
	versions = slices.Compact(versions)
	if len(versions) > constants.DefaultIncrementValue {
		return Warn(constants.CheckCRDs, fmt.Sprintf(constants.MsgCRDsMixed, strings.Join(versions, constants.ListSeparator)))
	}
	chart := constants.EmptyString
	if versions[constants.DefaultInitValue] != constants.UnknownValue {
		chart = fmt.Sprintf(constants.MsgCRDsVersion, versions[constants.DefaultInitValue])
	}
	return Pass(constants.CheckCRDs, fmt.Sprintf(constants.MsgCRDsOK, len(expectedCRDs), chart))
}

// The CRDs are read by name rather than by label, so CRDs a GitOps tool applied without the chart's labels count too.
func (r *runner) telarkCRDs(ctx context.Context) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	var list apiextensionsv1.CustomResourceDefinitionList
	if err := r.get(ctx, constants.CRDsPath, &list); err != nil {
		return nil, err
	}
	found := make(map[string]*apiextensionsv1.CustomResourceDefinition)
	for i := range list.Items {
		if list.Items[i].Spec.Group == base.Group {
			found[list.Items[i].Name] = &list.Items[i]
		}
	}
	return found, nil
}

func established(crd *apiextensionsv1.CustomResourceDefinition) bool {
	return slices.ContainsFunc(crd.Status.Conditions, func(condition apiextensionsv1.CustomResourceDefinitionCondition) bool {
		return condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue
	})
}

func (r *runner) workloads(ctx context.Context, release []appsv1.Deployment, selector metav1.ListOptions) Result {
	statefulSets, err := r.core.AppsV1().StatefulSets(r.namespace).List(ctx, selector)
	if err != nil {
		return ReadFailed(constants.CheckWorkloads, constants.ReadWorkloads, err)
	}
	var down []string
	for i := range release {
		down = appendDown(down, release[i].Name, release[i].Status.AvailableReplicas, release[i].Spec.Replicas)
	}
	for i := range statefulSets.Items {
		statefulSet := &statefulSets.Items[i]
		down = appendDown(down, statefulSet.Name, statefulSet.Status.ReadyReplicas, statefulSet.Spec.Replicas)
	}
	if len(down) > constants.DefaultInitValue {
		return Fail(constants.CheckWorkloads, fmt.Sprintf(constants.MsgWorkloadsDown, strings.Join(down, constants.ListSeparator)))
	}
	return Pass(constants.CheckWorkloads, fmt.Sprintf(constants.MsgWorkloadsOK, len(release), len(statefulSets.Items)))
}

func appendDown(down []string, name string, ready int32, replicas *int32) []string {
	want := ptr.Deref(replicas, constants.DefaultIncrementValue)
	if ready >= want {
		return down
	}
	return append(down, fmt.Sprintf(constants.MsgWorkloadReplicas, name, ready, want))
}

func (r *runner) restarts(ctx context.Context, selector metav1.ListOptions) Result {
	pods, err := r.core.CoreV1().Pods(r.namespace).List(ctx, selector)
	if err != nil {
		return ReadFailed(constants.CheckPodRestarts, constants.ReadPods, err)
	}
	since := time.Now().Add(-constants.RestartWindow)
	var restarted []string
	for i := range pods.Items {
		for _, status := range pods.Items[i].Status.ContainerStatuses {
			last := status.LastTerminationState.Terminated
			if status.RestartCount > constants.DefaultInitValue && last != nil && last.FinishedAt.After(since) {
				restarted = append(restarted, fmt.Sprintf(constants.MsgPodRestarts, pods.Items[i].Name, status.RestartCount, last.Reason))
			}
		}
	}
	if len(restarted) > constants.DefaultInitValue {
		return Warn(constants.CheckPodRestarts, fmt.Sprintf(constants.MsgRestartsSeen, strings.Join(restarted, constants.ListSeparator)))
	}
	return Pass(constants.CheckPodRestarts, constants.MsgRestartsNone)
}

func versionMatch(namespace string, telark []appsv1.Deployment) Result {
	client, server := version.Client().Version, version.Summarize(namespace, telark).Version
	if version.Skewed(client, server) {
		return Warn(constants.CheckVersionMatch, fmt.Sprintf(constants.WarnVersionSkew, client, server))
	}
	return Pass(constants.CheckVersionMatch, fmt.Sprintf(constants.MsgVersionsMatch, client, server))
}

// The admission controller is found by its chart name in any namespace, so a Kyverno the user runs counts too.
func (r *runner) kyverno(ctx context.Context) Result {
	webhooks, err := r.core.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx,
		metav1.ListOptions{LabelSelector: constants.KyvernoWebhookSelector})
	if err != nil {
		return ReadFailed(constants.CheckKyverno, constants.ReadWebhooks, err)
	}
	if len(webhooks.Items) == constants.DefaultInitValue {
		return Fail(constants.CheckKyverno, constants.MsgKyvernoNoWebhooks)
	}
	controllers, err := r.core.AppsV1().Deployments(metav1.NamespaceAll).List(ctx,
		metav1.ListOptions{LabelSelector: constants.LabelName + constants.SelectorEquals + constants.KyvernoControllerName})
	if err != nil {
		return ReadFailed(constants.CheckKyverno, constants.ReadKyvernoController, err)
	}
	if !slices.ContainsFunc(controllers.Items, serving) {
		return Fail(constants.CheckKyverno, constants.MsgKyvernoNoControl)
	}
	return Pass(constants.CheckKyverno, constants.MsgKyvernoOK)
}

func serving(deployment appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas > constants.DefaultInitValue
}

func (r *runner) crdGuard(ctx context.Context) Result {
	admission := r.core.AdmissionregistrationV1()
	_, err := admission.ValidatingAdmissionPolicies().Get(ctx, constants.CRDGuardPolicy, metav1.GetOptions{})
	var binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding
	if err == nil {
		binding, err = admission.ValidatingAdmissionPolicyBindings().Get(ctx, constants.CRDGuardBinding, metav1.GetOptions{})
	}
	switch {
	case apierrors.IsNotFound(err):
		return Warn(constants.CheckCRDGuard, constants.MsgGuardOff)
	case err != nil:
		return ReadFailed(constants.CheckCRDGuard, constants.ReadCRDGuard, err)
	case slices.Contains(binding.Spec.ValidationActions, admissionregistrationv1.Deny):
		return Pass(constants.CheckCRDGuard, constants.MsgGuardEnforced)
	}
	return Warn(constants.CheckCRDGuard, constants.MsgGuardAudits)
}

// RecoveryRequest is the recovery-request check alone, which an upgrade runs first. The last result is that check,
// or the one failure that stood for both.
func RecoveryRequest(ctx context.Context, client kubernetes.Interface, namespace string) Result {
	r := &runner{core: client, namespace: namespace}
	telark, _, _ := r.deployments(ctx)
	results := r.bootstrap(ctx, telark)
	return results[len(results)-constants.DefaultIncrementValue]
}

// The bootstrap and the recovery request are two checks on one read, so a failed read fails once.
func (r *runner) bootstrap(ctx context.Context, telark []appsv1.Deployment) []Result {
	var config telarkConfigObject
	path := telarkPath(r.namespace, v1alpha1.PluralTelarkConfigs) + constants.PathSeparator + telarkconfig.TelarkConfigResourceName
	err := r.get(ctx, path, &config)
	if apierrors.IsNotFound(err) {
		return []Result{Fail(constants.CheckBootstrap, constants.MsgConfigMissing)}
	}
	if err != nil {
		return []Result{ReadFailed(constants.CheckBootstrap, constants.ReadTelarkConfig, err)}
	}
	status := &config.Status.Bootstrap
	pinned := Pass(constants.CheckBootstrap, fmt.Sprintf(constants.MsgBootstrapPinned, status.Email))
	if !status.Pinned() {
		email := cmp.Or(exporterEnv(telark, constants.EnvBootstrapAdmin), constants.PlaceholderEmail)
		pinned = Warn(constants.CheckBootstrap, fmt.Sprintf(constants.MsgBootstrapUnpinned, email))
	}
	return []Result{pinned, recovery(status, telark)}
}

func recovery(status *telarkconfig.BootstrapStatus, telark []appsv1.Deployment) Result {
	request := status.Newest()
	if request == nil || !request.Open() {
		return Pass(constants.CheckRecoveryRequest, constants.MsgRecoveryNone)
	}
	hours, err := strconv.Atoi(exporterEnv(telark, constants.EnvRecoveryWaitHours))
	if err != nil {
		hours = constants.DefaultRecoveryWaitHour
	}
	completes := constants.UnknownValue
	if opened, err := time.Parse(time.RFC3339, request.OpenedAt); err == nil {
		completes = opened.Add(time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339)
	}
	return Warn(constants.CheckRecoveryRequest, fmt.Sprintf(constants.MsgRecoveryOpen, request.ID, request.Kind, completes))
}

func exporterEnv(telark []appsv1.Deployment, name string) string {
	for i := range telark {
		if telark[i].Labels[constants.LabelComponent] != constants.ExporterComponent {
			continue
		}
		for _, env := range telark[i].Spec.Template.Spec.Containers[constants.DefaultInitValue].Env {
			if env.Name == name {
				return env.Value
			}
		}
	}
	return constants.EmptyString
}

func (r *runner) plans(ctx context.Context) Result {
	var list protectionPlanList
	if err := r.get(ctx, telarkPath(r.namespace, v1alpha1.PluralProtectionPlans), &list); err != nil {
		return ReadFailed(constants.CheckProtectionPlans, constants.ReadProtectionPlans, err)
	}
	var unhealthy []string
	for _, plan := range list.Items {
		if plan.Status.Health == plans.HealthDrifted || plan.Status.Health == plans.HealthDegraded {
			unhealthy = append(unhealthy, plan.Metadata.Name)
		}
	}
	if len(unhealthy) > constants.DefaultInitValue {
		return Warn(constants.CheckProtectionPlans,
			fmt.Sprintf(constants.MsgPlansUnhealthy, len(unhealthy), strings.Join(unhealthy, constants.ListSeparator)))
	}
	return Pass(constants.CheckProtectionPlans, constants.MsgPlansHealthy)
}

func telarkPath(namespace, plural string) string {
	return fmt.Sprintf(constants.TelarkResourcePath, base.Group, base.V1Alpha1, namespace, plural)
}

// Telark's CRDs and custom resources are read raw through the clientset, so the CLI needs no dynamic client.
func (r *runner) get(ctx context.Context, path string, into any) error {
	raw, err := r.core.Discovery().RESTClient().Get().AbsPath(path).DoRaw(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}
