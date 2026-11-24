package testutil

import (
	"time"

	"github.com/telark/telark/cmd/telark/internal/constants"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	namespace      = "telark"
	cliVersion     = "0.0.0-dev"
	crdVersion     = "0.2.1"
	partOf         = "telark"
	exporter       = "exporter-service"
	auth           = "auth-service"
	controller     = "admission-controller"
	redis          = "telark-redis-master"
	authPod        = "telark-auth-service-0"
	adminEmail     = "test@example.com"
	kyvernoWebhook = "kyverno-resource-validating-webhook-cfg"
	guardPolicy    = "telark-crd-guard"
	guardBinding   = "telark-crd-guard-binding"
	telarkGroup    = "telark.io/v1alpha1"
	configKind     = "TelarkConfig"
	planKind       = "ProtectionPlan"
	configName     = "default"
	pinnedAt       = "2026-01-01T00:00:00Z"
	envAdmin       = "BOOTSTRAP_ADMIN"
	envWait        = "AUTH_REQUEST_WAIT_HOURS"
	keyBootstrap   = "bootstrap"
	keyHealth      = "health"
	healthy        = "healthy"
	nameLabel      = "app.kubernetes.io/name"
	kyvernoName    = "kyverno-admission-controller"
	webhookLabel   = "webhook.kyverno.io/managed-by"
	telarkSuffix   = ".telark.io"
	nodeName       = "node-1"
	classes        = "standard"
	ready          = 1
)

var plurals = []string{
	"applications", "protectionplans", "telarkconfigs", "categories", "users", "groups", "accessroles", "passkeys", "sessions",
	"authrequests",
}

func HealthyTelark() []runtime.Object {
	kyverno := Deployment(namespace, controller, "telark-kyverno", "3.9.2", "reg.kyverno.io/kyverno/kyverno:v1.19.2")
	kyverno.Labels[nameLabel] = kyvernoName
	return append(CRDs(crdVersion),
		ExporterDeployment(cliVersion, corev1.EnvVar{Name: envAdmin, Value: adminEmail}, corev1.EnvVar{Name: envWait, Value: "48"}),
		Deployment(namespace, auth, partOf, cliVersion, "ghcr.io/telark/auth:0.0.0"),
		kyverno,
		StatefulSet(namespace, redis, ready),
		Pod(namespace, authPod, noRestarts, time.Time{}, ""),
		KyvernoWebhook(kyvernoWebhook),
		&admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: guardPolicy}},
		GuardBinding(admissionregistrationv1.Deny),
		TelarkConfig(map[string]any{"email": adminEmail, "pinnedAt": pinnedAt}),
		ProtectionPlan("release-freeze", healthy),
	)
}

func CRDs(version string) []runtime.Object {
	objects := make([]runtime.Object, constants.DefaultInitValue, len(plurals))
	for _, plural := range plurals {
		objects = append(objects, CRD(plural+telarkSuffix, version, true))
	}
	return objects
}

func ExporterDeployment(version string, env ...corev1.EnvVar) *appsv1.Deployment {
	deployment := Deployment(namespace, exporter, partOf, version, "ghcr.io/telark/exporter:0.0.0")
	deployment.Spec.Template.Spec.Containers[constants.DefaultInitValue].Env = env
	return deployment
}

func KyvernoWebhook(name string) *admissionregistrationv1.ValidatingWebhookConfiguration {
	return &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{
		Name: name, Labels: map[string]string{webhookLabel: "kyverno"},
	}}
}

func GuardBinding(actions ...admissionregistrationv1.ValidationAction) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: guardBinding},
		Spec:       admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: guardPolicy, ValidationActions: actions},
	}
}

func TelarkConfig(bootstrap any) runtime.Object {
	return Resource(telarkGroup, configKind, namespace, configName, map[string]any{keyStatus: map[string]any{keyBootstrap: bootstrap}})
}

func ProtectionPlan(name, health string) runtime.Object {
	return Resource(telarkGroup, planKind, namespace, name, map[string]any{keyStatus: map[string]any{keyHealth: health}})
}

func FreshCluster() []runtime.Object {
	return []runtime.Object{Node(nodeName, "4", "8Gi", true), DefaultStorageClass(classes)}
}
