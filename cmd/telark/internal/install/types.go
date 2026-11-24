package install

import (
	"github.com/telark/telark/cmd/telark/internal/kube"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/registry"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/client-go/kubernetes"
)

type installOptions struct {
	kube           *kube.Flags
	values         values.Options
	bootstrapAdmin string
	mode           string
	version        string
	chartDirectory string
	wait           bool
	dryRun         bool
}

type upgradeOptions struct {
	kube           *kube.Flags
	values         values.Options
	version        string
	chartDirectory string
	resetValues    bool
	wait           bool
	dryRun         bool
}

type source struct {
	version   string
	directory string
	registry  *registry.Client
}

type objectCheck struct {
	core      kubernetes.Interface
	namespace string
	version   string
	total     int
	invalid   []string
	pruned    []string
}

type crdSchema struct {
	plural     string
	structural *structuralschema.Structural
	validator  validation.SchemaValidator
	rules      *cel.Validator
}

type planList struct {
	Items []plan `json:"items"`
}

type plan struct {
	Spec   planSpec   `json:"spec"`
	Status planStatus `json:"status"`
}

type planSpec struct {
	Mode string `json:"mode"`
}

type planStatus struct {
	Phase string `json:"phase"`
}

type manifestHead struct {
	Kind string `json:"kind"`
}
