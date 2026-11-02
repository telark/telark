package resources

const (
	FieldKind         = "kind"
	FieldSpec         = "spec"
	FieldTemplate     = "template"
	FieldJobTemplate  = "jobTemplate"
	FieldContainers   = "containers"
	FieldEnvFrom      = "envFrom"
	FieldEnv          = "env"
	FieldValueFrom    = "valueFrom"
	FieldConfigMapRef = "configMapRef"
	FieldSecretRef    = "secretRef"
	FieldSecretKeyRef = "secretKeyRef"
	FieldName         = "name"
	FieldKey          = "key"
	FieldVolumes      = "volumes"
	FieldConfigMap    = "configMap"
	FieldSecret       = "secret"
	FieldSecretName   = "secretName"

	ExpectedConfigMapRefs        = "configmaps: %v"
	ExpectedSecretRefs           = "secrets: %v"
	ExpectedCronJobConfigMapRefs = "cronjob configmaps: %v"
)

const (
	TestClaimNamespace     = "ops"
	TestClaimName          = "data-claim"
	TestClaimMissing       = "absent-claim"
	TestClaimCapacity      = "512Mi"
	TestClaimCapacityBytes = 512 << 20
	TestClaimBadCapacity   = "lots"

	FieldAPIVersion = "apiVersion"
	FieldMetadata   = "metadata"
	FieldNamespace  = "namespace"
	FieldStatus     = "status"
	FieldCapacity   = "capacity"
	FieldStorage    = "storage"
	ClaimAPIVersion = "v1"
	ClaimKind       = "PersistentVolumeClaim"

	ExpectedClaimCapacity = "capacity(%s) = %d, %v; want %d, error %v"
)
