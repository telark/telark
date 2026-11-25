package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/check"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"helm.sh/helm/v4/pkg/action"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

var errCopyJobFailed = errors.New(constants.ErrStorageJobFailed)

// The chart names new claims when the storage class or access mode changes and only mounts them, so the data is
// copied here first: the claims the target's exporter mounts are compared with the ones the running exporter mounts.
func planStorage(ctx context.Context, cfg *action.Configuration, core kubernetes.Interface, installed *releasev1.Release,
	manifest string) (*storageCopy, []check.Result) {
	target, rendered := exporterDeployment(manifest)
	if !rendered {
		return nil, nil
	}
	live, err := core.AppsV1().Deployments(installed.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []check.Result{check.ReadFailed(constants.CheckStorage, constants.ReadExporter, err)}
	}
	moves := storageMoves(target, live)
	if len(moves) == constants.DefaultInitValue {
		return nil, nil
	}
	exporter := runningExporter(cfg, installed, target.Name, moves)
	if exporter == constants.EmptyString {
		return nil, []check.Result{check.Fail(constants.CheckStorage, fmt.Sprintf(constants.MsgStorageUnmanaged, target.Name))}
	}
	plan := &storageCopy{cfg: cfg, core: core, namespace: installed.Namespace, force: forced(installed), name: target.Name,
		exporter: exporter, selector: labels.Set(live.Spec.Selector.MatchLabels).String(), job: copyJob(target, moves)}
	summary := make([]string, constants.DefaultInitValue, len(moves))
	for _, move := range moves {
		plan.claims = append(plan.claims, document(manifest, constants.KindClaim, move.to))
		summary = append(summary, fmt.Sprintf(constants.MsgStorageMove, move.from, move.to))
	}
	plan.summary = strings.Join(summary, constants.ListSeparator)
	return plan, []check.Result{check.Warn(constants.CheckStorage, fmt.Sprintf(constants.MsgStorageMoves, plan.summary))}
}

// Another kind can type a field differently than a Deployment does, so a document that doesn't decode is skipped.
func exporterDeployment(manifest string) (*appsv1.Deployment, bool) {
	for _, text := range releaseutil.SplitManifests(manifest) {
		var deployment appsv1.Deployment
		if yaml.Unmarshal([]byte(text), &deployment) == nil && deployment.Kind == constants.KindDeployment &&
			deployment.Labels[constants.LabelComponent] == constants.ExporterComponent {
			return &deployment, true
		}
	}
	return nil, false
}

func document(manifest, kind, name string) string {
	for _, text := range releaseutil.SplitManifests(manifest) {
		var head manifestHead
		if yaml.Unmarshal([]byte(text), &head) == nil && head.Kind == kind && head.Metadata.Name == name {
			return text
		}
	}
	return constants.EmptyString
}

// A failed upgrade leaves a last revision the exporter may not run, and applying that one would start the exporter
// on claims the copy hasn't filled: the document comes from a revision that mounts the claims the data leaves.
func runningExporter(cfg *action.Configuration, installed *releasev1.Release, name string, moves []storageMove) string {
	manifests := []string{installed.Manifest}
	if deployed, err := accessor(cfg.Releases.Deployed(constants.ReleaseName)); err == nil {
		manifests = append(manifests, deployed.Manifest())
	}
	for _, manifest := range manifests {
		text := document(manifest, constants.KindDeployment, name)
		var deployment appsv1.Deployment
		if yaml.Unmarshal([]byte(text), &deployment) == nil && text != constants.EmptyString &&
			!slices.ContainsFunc(moves, func(move storageMove) bool { return claimOf(&deployment, move.volume) != move.from }) {
			return text
		}
	}
	return constants.EmptyString
}

func storageMoves(target, live *appsv1.Deployment) []storageMove {
	var moves []storageMove
	for _, mount := range target.Spec.Template.Spec.Containers[constants.DefaultInitValue].VolumeMounts {
		from, to := claimOf(live, mount.Name), claimOf(target, mount.Name)
		if from != constants.EmptyString && to != constants.EmptyString && from != to {
			moves = append(moves, storageMove{volume: mount.Name, path: mount.MountPath, from: from, to: to})
		}
	}
	return moves
}

func claimOf(deployment *appsv1.Deployment, volume string) string {
	for _, candidate := range deployment.Spec.Template.Spec.Volumes {
		if candidate.Name == volume && candidate.PersistentVolumeClaim != nil {
			return candidate.PersistentVolumeClaim.ClaimName
		}
	}
	return constants.EmptyString
}

// The Job runs the target's exporter image, which ships the copy binary, as the exporter's pod would run.
func copyJob(target *appsv1.Deployment, moves []storageMove) *batchv1.Job {
	pod := target.Spec.Template.Spec
	exporter := pod.Containers[constants.DefaultInitValue]
	container := corev1.Container{Name: constants.StorageCopyName, Image: exporter.Image, ImagePullPolicy: exporter.ImagePullPolicy,
		Command: []string{constants.StorageCopyCommand}, Resources: exporter.Resources, SecurityContext: exporter.SecurityContext}
	volumes := make([]corev1.Volume, constants.DefaultInitValue, len(moves)*constants.StorageVolumesPerMove)
	for _, move := range moves {
		previous, source := constants.StoragePreviousPath+move.path, constants.StoragePreviousVolume+move.volume
		container.Args = append(container.Args, previous, move.path)
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{Name: source, MountPath: previous, ReadOnly: true}, corev1.VolumeMount{Name: move.volume, MountPath: move.path})
		volumes = append(volumes, claimVolume(source, move.from, true), claimVolume(move.volume, move.to, false))
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{GenerateName: constants.StorageCopyJobPrefix, Namespace: target.Namespace,
			Labels: map[string]string{constants.LabelName: constants.StorageCopyName}},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To[int32](constants.StorageCopyBackoffLimit),
			TTLSecondsAfterFinished: ptr.To[int32](constants.StorageCopyJobTTLSeconds),
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: ptr.To(false),
				SecurityContext: pod.SecurityContext, ImagePullSecrets: pod.ImagePullSecrets, NodeSelector: pod.NodeSelector,
				Tolerations: pod.Tolerations, Containers: []corev1.Container{container}, Volumes: volumes,
			}},
		},
	}
}

func claimVolume(name, claim string, readOnly bool) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim, ReadOnly: readOnly}}}
}

// The exporter stays stopped from before the copy until the upgrade starts it on the new claims, so nothing writes
// to either side meanwhile. A failed copy starts it again on the old claims, which still hold everything.
func (s *storageCopy) run(ctx context.Context, stdout io.Writer) error {
	if s == nil {
		return nil
	}
	_, _ = fmt.Fprintf(stdout, constants.MsgStorageCopying, s.summary)
	if err := s.apply(strings.Join(s.claims, constants.DocumentSeparator)); err != nil {
		return fmt.Errorf(constants.ErrStorageClaims, err)
	}
	if err := s.apply(stopped(s.exporter)); err != nil {
		return fmt.Errorf(constants.ErrStorageStop, err)
	}
	if err := s.copy(ctx); err != nil {
		return errors.Join(fmt.Errorf(constants.ErrStorageCopy, err), s.apply(s.exporter))
	}
	_, _ = io.WriteString(stdout, constants.MsgStorageCopied)
	return nil
}

// Applied as Helm applies the release, under its field manager, so the upgrade meets no field another manager owns.
func (s *storageCopy) apply(documents string) error {
	targets, err := s.cfg.KubeClient.Build(strings.NewReader(documents), false)
	for _, target := range targets {
		err = errors.Join(err, applyObject(target, s.namespace, s.force, false))
	}
	return err
}

// The document is one Helm rendered, so it reads back as it was written.
func stopped(exporter string) string {
	object := map[string]any{}
	_ = yaml.Unmarshal([]byte(exporter), &object)
	_ = unstructured.SetNestedField(object, int64(constants.NoReplicas), constants.KeySpec, constants.KeyReplicas)
	scaled, _ := yaml.Marshal(object)
	return string(scaled)
}

func (s *storageCopy) copy(ctx context.Context) error {
	pods := s.core.CoreV1().Pods(s.namespace)
	err := wait.PollUntilContextTimeout(ctx, constants.StoragePollInterval, constants.StorageStopTimeout, true,
		func(ctx context.Context) (bool, error) {
			running, err := pods.List(ctx, metav1.ListOptions{LabelSelector: s.selector})
			return err == nil && len(running.Items) == constants.DefaultInitValue, err
		})
	if err != nil {
		return err
	}
	jobs := s.core.BatchV1().Jobs(s.namespace)
	job, err := jobs.Create(ctx, s.job, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	err = wait.PollUntilContextTimeout(ctx, constants.StoragePollInterval, constants.StorageCopyTimeout, true,
		func(ctx context.Context) (bool, error) { return s.copied(ctx, job.Name) })
	if err != nil && !errors.Is(err, errCopyJobFailed) {
		// A copy that may still run must end before the exporter starts again. A failed one stays for its logs.
		_ = jobs.Delete(context.WithoutCancel(ctx), job.Name, metav1.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationBackground)})
	}
	return err
}

func (s *storageCopy) copied(ctx context.Context, name string) (bool, error) {
	job, err := s.core.BatchV1().Jobs(s.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(job.Status.Conditions, func(condition batchv1.JobCondition) bool {
		return condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue
	}) {
		return false, fmt.Errorf(constants.ErrStorageJobLogs, errCopyJobFailed, s.namespace, name)
	}
	return job.Status.Succeeded > constants.DefaultInitValue, nil
}

// Helm can fail before it reaches the exporter, which would then stay stopped: it starts again on its old claims.
func (s *storageCopy) recover(ctx context.Context) error {
	if s == nil {
		return nil
	}
	live, err := s.core.AppsV1().Deployments(s.namespace).Get(context.WithoutCancel(ctx), s.name, metav1.GetOptions{})
	if err != nil || ptr.Deref(live.Spec.Replicas, constants.DefaultIncrementValue) != constants.NoReplicas {
		return err
	}
	return s.apply(s.exporter)
}
