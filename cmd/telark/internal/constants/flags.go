package constants

const (
	FlagKubeconfig      = "kubeconfig"
	FlagKubeconfigUsage = "Path to the kubeconfig file (default: the files in $KUBECONFIG, else ~/.kube/config)"
	FlagContext         = "context"
	FlagContextUsage    = "The kubeconfig context to use (default: the current context)"
	FlagNamespace       = "namespace"
	FlagNamespaceShort  = "n"
	FlagNamespaceUsage  = "The namespace Telark runs in"
)

const (
	FlagOutput      = "output"
	FlagOutputShort = "o"
	FlagOutputUsage = "Output format: table, json or yaml"
	FlagOutputType  = "format"
	FlagClient      = "client"
	FlagClientUsage = "Show only the CLI version, without contacting the cluster"
)

const (
	FlagPre                  = "pre"
	FlagPreUsage             = "Check the cluster before an install instead of a running Telark"
	FlagWait                 = "wait"
	FlagWaitUsage            = "Repeat the checks until they all pass or this much time has passed (default: run once)"
	FlagSkipUpdateCheck      = "skip-update-check"
	FlagSkipUpdateCheckUsage = "Don't ask GitHub whether a newer release exists (or set " + EnvSkipUpdateCheck + "=1)"
	FlagMode                 = "mode"
	FlagModeUsage            = "With --pre, the app.mode to size the nodes for: minimal, standard or performance"
)
