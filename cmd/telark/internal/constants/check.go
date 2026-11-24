package constants

import "time"

const (
	CheckUse   = "check"
	CheckShort = "Check a Telark installation, or a cluster before an install"
	CheckLong  = "telark check prints one line per check and exits 1 when any check fails. " +
		"With --pre it checks the cluster before an install instead."
)

const (
	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"
)

const (
	CheckKubernetesVersion = "kubernetes-version"
	CheckPermissions       = "permissions"
	CheckStorageClass      = "storage-class"
	CheckKyverno           = "kyverno"
	CheckEarlierInstall    = "earlier-install"
	CheckNodeCapacity      = "node-capacity"
	CheckCRDs              = "crds"
	CheckWorkloads         = "workloads"
	CheckPodRestarts       = "pod-restarts"
	CheckCRDGuard          = "crd-guard"
	CheckBootstrap         = "bootstrap"
	CheckRecoveryRequest   = "recovery-request"
	CheckProtectionPlans   = "protection-plans"
	CheckVersionMatch      = "version-match"
	CheckUpdate            = "update"
)

const (
	MsgReadFailed        = "Couldn't read %s: %v"
	MsgKubeVersionOK     = "Kubernetes %s is 1.30 or newer"
	MsgKubeVersionOld    = "Kubernetes %s is older than 1.30, the oldest release Telark supports"
	MsgPermissionsOK     = "You can create the %d kinds of objects the chart installs"
	MsgPermissionsDenied = "You can't create %s, which the chart installs"
	MsgStorageClassOK    = "%s is the default StorageClass, for the volume claims"
	MsgStorageClassNone  = "No default StorageClass, so the volume claims would stay Pending: " +
		"mark a class as the default, or set app.persistence.storageClass"
	MsgKyvernoAbsent       = "No other Kyverno runs in the cluster"
	MsgKyvernoToUse        = "Kyverno runs in the cluster, and Telark will use it (kyverno.enabled=false)"
	MsgKyvernoNeeded       = "kyverno.enabled=false needs a Kyverno that already runs, and none has its webhooks registered"
	MsgStorageClassFound   = "%s exists, for the volume claims"
	MsgStorageClassMissing = "app.persistence.storageClass=%q doesn't exist in this cluster (kubectl get storageclass lists them)"
	MsgStorageClassNoClass = "app.persistence.storageClass=- binds the volume claims to volumes you create"
	MsgKyvernoPresent      = "Kyverno already runs in this cluster (%s): to use it, install with kyverno.enabled=false"
	MsgInstallRunning      = "Telark %s already runs in namespace %q: upgrade it instead of installing"
	MsgInstallLeftovers    = "An earlier install left %s: a reinstall keeps this data"
	MsgLeftoverCRDs        = "Telark's CRDs (%d)"
	MsgLeftoverClaims      = "volume claims (%d) in namespace %q"
	MsgLeftoverJoin        = " and "
	MsgInstallClean        = "No data from an earlier install"
	MsgCapacityOK          = "The nodes have %.2f CPU and %.1f GiB free, enough for the %s mode's %.2f CPU and %.1f GiB"
	MsgCapacityShort       = "The nodes have %.2f CPU and %.1f GiB free, and the %s mode requests %.2f CPU and %.1f GiB: " +
		"some pods stay Pending until the cluster adds nodes"
	MsgCRDsOK             = "%d CRDs are established%s"
	MsgCRDsVersion        = " (telark-crds %s)"
	MsgCRDsMissing        = "CRDs missing or not established: %s"
	MsgCRDsMixed          = "The CRDs come from several chart versions: %s"
	MsgWorkloadsOK        = "Every Deployment (%d) and StatefulSet (%d) is available"
	MsgWorkloadsDown      = "Not available: %s"
	MsgWorkloadReplicas   = "%s (%d of %d)"
	MsgRestartsNone       = "No pod restarted in the last hour"
	MsgRestartsSeen       = "Pods restarted in the last hour: %s"
	MsgPodRestarts        = "%s (%d, %s)"
	MsgKyvernoOK          = "Kyverno's admission controller is available and its webhooks are registered"
	MsgKyvernoNoWebhooks  = "Kyverno has no webhooks registered, so no protection plan is enforced"
	MsgKyvernoNoControl   = "Kyverno's admission controller isn't available"
	MsgGuardEnforced      = "The CRD guard is enforced"
	MsgGuardAudits        = "The CRD guard only audits: set app.security.crdGuard.enforce=true to enforce it"
	MsgGuardOff           = "The CRD guard is off, so any account that may write Telark's resources can change them directly"
	MsgBootstrapPinned    = "The bootstrap account %s is pinned"
	MsgBootstrapUnpinned  = "The bootstrap account isn't pinned yet: run `telark auth enroll --email %s`"
	MsgConfigMissing      = "TelarkConfig default doesn't exist yet: the exporter creates it when it starts"
	MsgRecoveryNone       = "No recovery request is open"
	MsgRecoveryOpen       = "Request %s (%s) is open and can complete at %s"
	MsgPlansHealthy       = "No protection plan is drifted or degraded"
	MsgPlansUnhealthy     = "Protection plans drifted or degraded (%d): %s"
	MsgVersionsMatch      = "The CLI (%s) and the server (%s) are from the same release"
	MsgUpdateAvailable    = "Telark %s is out, and this CLI is %s: %s"
	MsgUpToDate           = "This CLI (%s) is the latest release"
	MsgUpdateSkipped      = "The update check was skipped: %v"
	MsgWaiting            = "Waiting up to %s for every check to pass\n"
	ErrChecksFailed       = "%d of %d checks failed"
	ErrUnknownMode        = "--mode %q: must be minimal, standard or performance"
	ErrModeNeedsPre       = "--mode works only with --pre"
	ErrReleasesStatus     = "GitHub answered %s"
	ReadServerVersion     = "the server version"
	ReadPermissions       = "your permissions"
	ReadStorageClasses    = "the StorageClasses"
	ReadWebhooks          = "Kyverno's webhooks"
	ReadCRDs              = "the CRDs"
	ReadDeployments       = "Telark's Deployments"
	ReadVolumeClaims      = "the volume claims"
	ReadNodes             = "the nodes"
	ReadPods              = "the pods"
	ReadWorkloads         = "the workloads"
	ReadKyvernoController = "Kyverno's admission controller"
	ReadCRDGuard          = "the CRD guard"
	ReadTelarkConfig      = "TelarkConfig default"
	ReadProtectionPlans   = "the protection plans"
	PlaceholderEmail      = "<email>"
)

const (
	MinKubernetesVersion = "1.30.0"
	VerbCreate           = "create"
	ResourceNamespaces   = "namespaces"
	GroupSeparator       = "."
	// Every kind `helm template` renders from the chart's defaults, as resource.group, the core group without one.
	ChartNamespacedResources = "configmaps persistentvolumeclaims secrets services serviceaccounts deployments.apps " +
		"statefulsets.apps jobs.batch horizontalpodautoscalers.autoscaling networkpolicies.networking.k8s.io " +
		"poddisruptionbudgets.policy roles.rbac.authorization.k8s.io rolebindings.rbac.authorization.k8s.io"
	ChartClusterResources = "customresourcedefinitions.apiextensions.k8s.io clusterroles.rbac.authorization.k8s.io " +
		"clusterrolebindings.rbac.authorization.k8s.io validatingadmissionpolicies.admissionregistration.k8s.io " +
		"validatingadmissionpolicybindings.admissionregistration.k8s.io apiservices.apiregistration.k8s.io"
)

const (
	ModeMinimal     = "minimal"
	ModeStandard    = "standard"
	ModePerformance = "performance"
	// The requests of every pod `helm template` renders from the chart's defaults in each mode, Ollama included.
	MinimalCPUMillis     = 1350
	MinimalMemoryMiB     = 3016
	StandardCPUMillis    = 1650
	StandardMemoryMiB    = 3400
	PerformanceCPUMillis = 2550
	PerformanceMemoryMiB = 5704
	MillisPerCPU         = 1000.0
	MiBPerGiB            = 1024.0
	BytesPerMiB          = 1 << 20
)

const (
	LabelInstance           = "app.kubernetes.io/instance"
	LabelName               = "app.kubernetes.io/name"
	SelectorEquals          = "="
	KyvernoWebhookSelector  = "webhook.kyverno.io/managed-by=kyverno"
	KyvernoControllerName   = "kyverno-admission-controller"
	DefaultClassAnnotation  = "storageclass.kubernetes.io/is-default-class"
	BetaDefaultClassAnnot   = "storageclass.beta.kubernetes.io/is-default-class"
	AnnotationTrue          = "true"
	NoStorageClass          = "-"
	CRDGuardPolicy          = "telark-crd-guard"
	CRDGuardBinding         = "telark-crd-guard-binding"
	ExporterComponent       = "exporter-service"
	EnvBootstrapAdmin       = "BOOTSTRAP_ADMIN"
	EnvRecoveryWaitHours    = "AUTH_REQUEST_WAIT_HOURS"
	DefaultRecoveryWaitHour = 24
	CRDsPath                = "/apis/apiextensions.k8s.io/v1/customresourcedefinitions"
	TelarkResourcePath      = "/apis/%s/%s/namespaces/%s/%s"
	PathSeparator           = "/"
	TagPrefix               = "v"
)

const (
	CheckPollInterval  = 2 * time.Second
	RestartWindow      = time.Hour
	UpdateCheckTimeout = 2 * time.Second
	ReleasesURL        = "https://api.github.com/repos/telark/telark/releases/latest"
	HeaderAccept       = "Accept"
	HeaderUserAgent    = "User-Agent"
	GitHubJSON         = "application/vnd.github+json"
	UserAgentFormat    = "telark/%s"
	MaxReleaseBytes    = 1 << 20
	EnvSkipUpdateCheck = "TELARK_SKIP_UPDATE_CHECK"
)
