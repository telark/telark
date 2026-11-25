package install

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"helm.sh/helm/v4/pkg/action"
	helmkube "helm.sh/helm/v4/pkg/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	errNotConfirmed = errors.New(constants.ErrUninstallNotSure)
	cleanupTargets  = []cleanupTarget{
		{v1alpha1.PluralUsers, finalizers.UserCleanup},
		{v1alpha1.PluralGroups, finalizers.GroupCleanup},
		{v1alpha1.PluralAccessRoles, finalizers.RoleCleanup},
	}
)

func NewUninstallCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &uninstallOptions{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.UninstallUse,
		Short: constants.UninstallShort,
		Long:  constants.UninstallLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&opts.yes, constants.FlagYes, constants.FlagYesShort, false, constants.FlagYesUsage)
	flags.BoolVar(&opts.dryRun, constants.FlagDryRun, false, constants.FlagUninstallDryRun)
	flags.BoolVar(&opts.wait, constants.FlagHelmWait, false, constants.FlagUninstallWait)
	flags.DurationVar(&opts.timeout, constants.FlagTimeout, constants.HelmTimeout, constants.FlagTimeoutUsage)
	return cmd
}

// The release goes first: auth and the CRD write guard leave with it, so nothing restores a finalizer or refuses
// the patch. The finalizers are then removed as the caller, and a failure there never undoes the uninstall.
func (o *uninstallOptions) run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	core, objects, err := o.clients()
	if err != nil {
		return err
	}
	cfg := configuration(o.kube)
	installed, err := storedRelease(cfg)
	if err != nil {
		return err
	}
	held, unread := o.held(ctx, objects, io.Discard)
	if installed == nil && len(held)+unread == constants.DefaultInitValue {
		_, err := fmt.Fprintf(stdout, constants.MsgNotInstalled, o.kube.Namespace)
		return err
	}
	if installed == nil {
		_, _ = fmt.Fprintf(stdout, constants.MsgUninstallNoRelease, o.kube.Namespace)
	} else {
		_, _ = fmt.Fprintf(stdout, constants.MsgUninstallRelease, installed.Chart.Metadata.Version, o.kube.Namespace)
	}
	_, _ = fmt.Fprintf(stdout, constants.MsgUninstallHeld+constants.MsgUninstallKeeps, len(held))
	if o.dryRun {
		_, err := io.WriteString(stdout, constants.MsgDryRunUninstall)
		return err
	}
	if !o.yes && !confirmed(stdin, stdout) {
		return errNotConfirmed
	}
	if installed != nil {
		if err := o.uninstall(cfg); err != nil {
			return err
		}
		_, _ = io.WriteString(stdout, constants.MsgUninstalled)
	}
	return o.clear(ctx, core, objects, stdout, stderr)
}

func (o *uninstallOptions) clients() (kubernetes.Interface, dynamic.Interface, error) {
	objects, err := o.kube.Dynamic()
	if err != nil {
		return nil, nil, err
	}
	core, err := o.kube.Clientset()
	return core, objects, err
}

func confirmed(stdin io.Reader, stdout io.Writer) bool {
	_, _ = io.WriteString(stdout, constants.MsgUninstallConfirm)
	answer, _ := bufio.NewReader(stdin).ReadString(constants.LineEnd)
	return slices.Contains([]string{constants.AnswerY, constants.AnswerYes}, strings.ToLower(strings.TrimSpace(answer)))
}

func (o *uninstallOptions) uninstall(cfg *action.Configuration) error {
	uninstall := action.NewUninstall(cfg)
	uninstall.WaitStrategy, uninstall.Timeout = waitStrategy(o.wait), o.timeout
	uninstall.WaitOptions = []helmkube.WaitOption{helmkube.WithStatusComputeWorkers(constants.HelmStatusWorkers)}
	if _, err := uninstall.Run(constants.ReleaseName); err != nil {
		return fmt.Errorf(constants.ErrUninstall, err)
	}
	return nil
}

// auth's sweep restores a missing finalizer within a minute, so its pods have to be gone first.
func (o *uninstallOptions) clear(ctx context.Context, core kubernetes.Interface, objects dynamic.Interface, stdout, stderr io.Writer) error {
	selector := constants.LabelComponent + constants.SelectorEquals + constants.AuthComponent
	err := wait.PollUntilContextTimeout(ctx, constants.FinalizerPollInterval, o.timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := core.CoreV1().Pods(o.kube.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		return err == nil && len(pods.Items) == constants.DefaultInitValue, err
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, constants.WarnAuthStillRuns, err)
	}
	held, failed := o.held(ctx, objects, stderr)
	removed := constants.DefaultInitValue
	for _, holder := range held {
		kept := slices.DeleteFunc(slices.Clone(holder.finalizers), func(name string) bool { return name == holder.target.finalizer })
		patch, _ := json.Marshal(map[string]any{constants.KeyMetadata: map[string]any{constants.KeyFinalizers: kept}})
		_, err := o.resource(objects, holder.target).Patch(ctx, holder.name, types.MergePatchType, patch, metav1.PatchOptions{})
		// An object deleted since the list holds no finalizer anymore.
		if err != nil && !apierrors.IsNotFound(err) {
			_, _ = fmt.Fprintf(stderr, constants.WarnFinalizerRemove, holder.target.plural, holder.name, err)
			failed++
			continue
		}
		removed++
	}
	_, _ = fmt.Fprintf(stdout, constants.MsgFinalizersRemoved+constants.MsgUninstallNext, removed, failed)
	if failed > constants.DefaultInitValue {
		return fmt.Errorf(constants.ErrFinalizersLeft, failed)
	}
	return nil
}

// A kind whose CRD is gone holds nothing. Any other list failure counts as a finalizer left.
func (o *uninstallOptions) held(ctx context.Context, objects dynamic.Interface, stderr io.Writer) (held []holder, unread int) {
	for _, target := range cleanupTargets {
		list, err := o.resource(objects, target).List(ctx, metav1.ListOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, constants.WarnFinalizerList, target.plural, err)
			unread++
			continue
		}
		for i := range list.Items {
			if item := &list.Items[i]; slices.Contains(item.GetFinalizers(), target.finalizer) {
				held = append(held, holder{target: target, name: item.GetName(), finalizers: item.GetFinalizers()})
			}
		}
	}
	return held, unread
}

func (o *uninstallOptions) resource(objects dynamic.Interface, target cleanupTarget) dynamic.ResourceInterface {
	gvr := schema.GroupVersionResource{Group: base.Group, Version: base.V1Alpha1, Resource: target.plural}
	return objects.Resource(gvr).Namespace(o.kube.Namespace)
}
