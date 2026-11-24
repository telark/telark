package kube

import (
	"errors"
	"maps"
	"slices"

	"github.com/spf13/cobra"
	"github.com/telark/telark/cmd/telark/internal/constants"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var errNoCluster = errors.New(constants.ErrNoCluster)

func AddFlags(cmd *cobra.Command) *Flags {
	f := &Flags{}
	flags := cmd.PersistentFlags()
	flags.StringVar(&f.kubeconfig, constants.FlagKubeconfig, constants.EmptyString, constants.FlagKubeconfigUsage)
	flags.StringVar(&f.kubeContext, constants.FlagContext, constants.EmptyString, constants.FlagContextUsage)
	flags.StringVarP(&f.Namespace, constants.FlagNamespace, constants.FlagNamespaceShort, constants.DefaultNamespace, constants.FlagNamespaceUsage)
	_ = cmd.RegisterFlagCompletionFunc(constants.FlagContext, f.completeContexts)
	return f
}

func (f *Flags) Clientset() (kubernetes.Interface, error) {
	config, err := f.restConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// AuthRequests are created and watched as custom objects, which only the dynamic client decodes.
func (f *Flags) Dynamic() (dynamic.Interface, error) {
	config, err := f.restConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

// client-go's own message points at KUBERNETES_MASTER, which this CLI never reads.
func (f *Flags) restConfig() (*rest.Config, error) {
	config, err := f.clientConfig().ClientConfig()
	if clientcmd.IsEmptyConfig(err) {
		return nil, errNoCluster
	}
	if err != nil {
		return nil, err
	}
	// client-go's default of 5 requests a second would throttle check --pre's one access review per kind.
	config.QPS, config.Burst = constants.ClientQPS, constants.ClientBurst
	return config, nil
}

// Helm's SDK reads the cluster through cli-runtime, so it gets the same three flags.
func (f *Flags) RESTClientGetter() genericclioptions.RESTClientGetter {
	getter := genericclioptions.NewConfigFlags(true)
	getter.KubeConfig, getter.Context, getter.Namespace = &f.kubeconfig, &f.kubeContext, &f.Namespace
	getter.WrapConfigFn = func(config *rest.Config) *rest.Config {
		config.QPS, config.Burst = constants.ClientQPS, constants.ClientBurst
		return config
	}
	return getter
}

func (f *Flags) clientConfig() clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = f.kubeconfig
	overrides := &clientcmd.ConfigOverrides{CurrentContext: f.kubeContext}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
}

func (f *Flags) completeContexts(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	raw, err := f.clientConfig().RawConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return slices.Sorted(maps.Keys(raw.Contexts)), cobra.ShellCompDirectiveNoFileComp
}
