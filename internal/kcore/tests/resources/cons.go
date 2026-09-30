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
