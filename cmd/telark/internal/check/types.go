package check

import (
	"time"

	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Status string

type Result struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
}

type Report struct {
	Success bool     `json:"success"`
	Checks  []Result `json:"checks"`
}

// StorageClass and ExternalKyverno carry the install's values: app.persistence.storageClass and kyverno.enabled=false.
type Options struct {
	Pre             bool
	Wait            time.Duration
	Mode            string
	StorageClass    string
	ExternalKyverno bool
	UpdateCheck     bool
}

type options struct {
	kube       *kube.Flags
	format     output.Format
	pre        bool
	wait       time.Duration
	skipUpdate bool
	mode       string
}

type runner struct {
	core      kubernetes.Interface
	namespace string
	opts      Options
}

type modeSize struct {
	cpuMillis int64
	memoryMiB int64
}

type capacity struct {
	cpuMillis   int64
	memoryBytes int64
}

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type telarkConfigObject struct {
	Status telarkConfigStatus `json:"status"`
}

type telarkConfigStatus struct {
	Bootstrap telarkconfig.BootstrapStatus `json:"bootstrap"`
}

type protectionPlanList struct {
	Items []protectionPlanItem `json:"items"`
}

type protectionPlanItem struct {
	Metadata metav1.ObjectMeta    `json:"metadata"`
	Status   protectionPlanStatus `json:"status"`
}

type protectionPlanStatus struct {
	Health string `json:"health"`
}
