package auth

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func NewCommand(kubeFlags *kube.Flags) *cobra.Command {
	cmd := &cobra.Command{Use: constants.AuthUse, Short: constants.AuthShort, Long: constants.AuthLong}
	cmd.AddCommand(newEnrollCommand(kubeFlags), newStatusCommand(kubeFlags))
	return cmd
}

func newEnrollCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &options{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.EnrollUse,
		Short: constants.EnrollShort,
		Long:  constants.EnrollLong,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.enroll(cmd.Context(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.email, constants.FlagEmail, constants.EmptyString, constants.FlagEmailUsage)
	_ = cmd.MarkFlagRequired(constants.FlagEmail)
	return cmd
}

func newStatusCommand(kubeFlags *kube.Flags) *cobra.Command {
	opts := &options{kube: kubeFlags}
	cmd := &cobra.Command{
		Use:   constants.AuthStatusUse,
		Short: constants.AuthStatusShort,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.status(cmd.Context(), cmd.OutOrStdout())
		},
	}
	output.AddFormatFlag(cmd, &opts.format)
	return cmd
}

func (o *options) clients() (kubernetes.Interface, dynamic.Interface, error) {
	objects, err := o.kube.Dynamic()
	if err != nil {
		return nil, nil, err
	}
	core, err := o.kube.Clientset()
	return core, objects, err
}

// AuthRequest objects arrived in 0.3.0; an older Telark still runs the command in the auth pod.
func served(core kubernetes.Interface, oldCommand string) error {
	resources, err := core.Discovery().ServerResourcesForGroupVersion(requestsResource.GroupVersion().String())
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(constants.ErrReadAPI, err)
	}
	if err == nil && slices.ContainsFunc(resources.APIResources, func(resource metav1.APIResource) bool {
		return resource.Name == v1alpha1.PluralAuthRequests
	}) {
		return nil
	}
	return fmt.Errorf(constants.ErrNoAuthRequests, oldCommand)
}
