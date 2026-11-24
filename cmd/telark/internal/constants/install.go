package constants

import "time"

const (
	InstallUse   = "install"
	InstallShort = "Install Telark with Helm, after the pre-install checks"
	InstallLong  = "telark install runs the checks of telark check --pre, then installs the signed Telark chart as the Helm " +
		"release telark, so helm list, helm get values and helm upgrade keep working on it."
	UpgradeUse   = "upgrade"
	UpgradeShort = "Upgrade Telark with Helm, after the pre-upgrade checks"
	UpgradeLong  = "telark upgrade keeps the release's values, takes the new chart's defaults and runs the pre-upgrade " +
		"checks first. Afterward it waits for telark check to pass."
)

const (
	FlagBootstrapAdmin      = "bootstrap-admin"
	FlagBootstrapAdminUsage = "The email of the bootstrap admin, the account that signs in first (app.auth.bootstrap.admin)"
	FlagInstallModeUsage    = "The app.mode that sizes Telark: minimal, standard or performance"
	FlagValues              = "values"
	FlagValuesShort         = "f"
	FlagValuesUsage         = "A values file (repeatable)"
	FlagSet                 = "set"
	FlagSetUsage            = "A value as key=value (repeatable)"
	FlagChartVersion        = "version"
	FlagChartVersionUsage   = "The chart version (default: this CLI's version)"
	FlagChartDirectory      = "chart-directory"
	FlagChartDirectoryUsage = "A chart checkout to use instead of the registry, without the signature check"
	FlagHelmWait            = "wait"
	FlagHelmWaitUsage       = "Wait until every object is ready"
	FlagDryRun              = "dry-run"
	FlagDryRunUsage         = "Run the checks and render on the server without changing anything"
	FlagResetValues         = "reset-values"
	FlagResetValuesUsage    = "Start from the chart's defaults instead of the release's values"
)

const (
	CheckSignature     = "signature"
	CheckStep          = "step"
	CheckStoredObjects = "stored-objects"
	CheckServerDryRun  = "server-dry-run"
	CheckEnforcePlans  = "enforce-plans"
)

const (
	MsgSignatureValid       = "The chart's signature is valid: %s"
	MsgSignatureInvalid     = "The chart's signature doesn't verify: %s"
	MsgSignatureNoCosign    = "cosign isn't installed, so the chart's signature wasn't checked: install cosign to check it"
	MsgSignatureSkipped     = "The chart comes from %s, so its signature isn't checked"
	MsgStepSame             = "%s is the release already installed"
	MsgStepNext             = "%s → %s is the next release"
	MsgStepDowngrade        = "%s is older than the installed %s: go back with helm rollback instead"
	MsgStepSkips            = "%s → %s skips %s: upgrade one release at a time"
	MsgStepUnlisted         = "Couldn't list the published releases, so skipped ones aren't checked: %v"
	MsgObjectsFit           = "Every stored object (%d) fits the %s schemas"
	MsgObjectsInvalid       = "Stored objects that don't fit the %s schemas (%d): %s"
	MsgObjectsPruned        = "The %s schemas drop fields from stored objects (%d): %s"
	MsgObjectsNoCRDs        = "The chart installs no CRDs (crds.enabled=false), so the stored objects aren't checked"
	MsgObjectExample        = "%s/%s: %s"
	MsgServerAccepts        = "The server accepts every object of the upgrade (%d, dry run)"
	MsgServerRefusesObjects = "The server refuses %d of %d objects (dry run): %s"
	MsgServerRefuses        = "The server refuses the upgrade (dry run): %v"
	MsgNoEnforcePlans       = "No protection plan enforces now"
	MsgEnforcePlans         = "Protection plans enforce now (%d): Kyverno restarts during the upgrade, and its failure policy " +
		"decides what happens to changes meanwhile"
	MsgInstalled         = "Installed Telark %s in namespace %q.\nNext:\n  telark check\n  telark auth enroll --email %s\n"
	MsgUpgraded          = "Upgraded Telark to %s in namespace %q.\n"
	MsgDryRunInstall     = "Dry run of Telark %s in namespace %q: nothing changed.\nValues:\n"
	MsgDryRunObjects     = "Objects:\n"
	MsgDryRunUpgrade     = "Dry run of the upgrade to %s: nothing changed.\n"
	MsgObjectKind        = "  %-34s %d\n"
	ErrBootstrapAdmin    = "pass --bootstrap-admin <email>: the chart requires the bootstrap admin (app.auth.bootstrap.admin)"
	ErrNoRelease         = "no Telark release in namespace %q: install it with telark install"
	ErrPullChart         = "pull the chart %s: %w"
	ErrLoadChart         = "load the chart from %s: %w"
	ErrChartDependencies = "the chart in %s lacks its subcharts, so build them first (make deps, or helm dependency build): %w"
	ErrInstallMode       = "app.mode %q: must be minimal, standard or performance"
	ErrHelmInit          = "reach Helm's release storage: %w"
	ErrInstall           = "install: %w"
	ErrUpgrade           = "upgrade: %w"
	ReadStoredObjects    = "the stored objects"
)

const (
	ReleaseName                = "telark"
	ChartRepository            = "ghcr.io/telark/charts/telark"
	ChartTagSeparator          = ":"
	ChartDigestSeparator       = "@"
	HelmFieldManager           = "helm"
	ManagedByLabel             = "app.kubernetes.io/managed-by"
	ManagedByHelm              = "Helm"
	ReleaseNameAnnotation      = "meta.helm.sh/release-name"
	ReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
	HelmDriver                 = "secret"
	HelmTimeout                = 5 * time.Minute
	HelmStatusWorkers          = 8
	UpgradeCheckWait           = 5 * time.Minute
	ServerSideApplyAuto        = "auto"
	CRDKind                    = "CustomResourceDefinition"
	MaxExamples                = 3
	CosignCommand              = "cosign"
	CosignVerify               = "verify"
	CosignIdentityFlag         = "--certificate-identity"
	CosignIdentity             = "https://github.com/telark/telark/.github/workflows/release-charts.yaml@refs/tags/v%s"
	CosignIssuerFlag           = "--certificate-oidc-issuer"
	CosignIssuer               = "https://token.actions.githubusercontent.com"
	KeyApp                     = "app"
	KeyAuth                    = "auth"
	KeyBootstrap               = "bootstrap"
	KeyAdmin                   = "admin"
	KeyMode                    = "mode"
	KeyPersistence             = "persistence"
	KeyStorageClass            = "storageClass"
	KeyKyverno                 = "kyverno"
	KeyEnabled                 = "enabled"
	ValueAdmin                 = "app.auth.bootstrap.admin=%s"
	ValueMode                  = "app.mode=%s"
)
