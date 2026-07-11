package plans

import (
	"fmt"

	"github.com/telark/data/constants"
)

type ParamType string

const (
	ParamTypeStringArray ParamType = "string-array"
)

type ScopeSupport string

const (
	ScopeSupportNamespaces   ScopeSupport = "namespaces"
	ScopeSupportApplications ScopeSupport = "applications"
)

type ParamSpec struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        ParamType `json:"type"`
	Required    bool      `json:"required"`
	Placeholder string    `json:"placeholder,omitempty"`
	Description string    `json:"description,omitempty"`
}

type Template struct {
	ID              string         `json:"id"`
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	SupportedScopes []ScopeSupport `json:"supportedScopes"`
	Params          []ParamSpec    `json:"params"`
}

var Templates = []Template{
	{
		ID:              "block-create",
		Code:            "bc",
		Name:            "Block Resource Creation",
		Description:     "Prevents creation of any Kubernetes resources within the target scope.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-update",
		Code:            "bu",
		Name:            "Block Resource Updates",
		Description:     "Prevents updates to any Kubernetes resources within the target scope.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-delete",
		Code:            "bd",
		Name:            "Block Resource Deletion",
		Description:     "Prevents deletion of any Kubernetes resources within the target scope.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-image-types",
		Code:            "bit",
		Name:            "Block Image Patterns",
		Description:     "Prevents workloads from using container images matching the specified patterns.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params: []ParamSpec{
			{
				Key:         "imagePatterns",
				Label:       "Image Patterns",
				Type:        ParamTypeStringArray,
				Required:    true,
				Placeholder: "e.g. */untrusted-repo/*",
				Description: "Glob patterns for blocked image registries or image names.",
			},
		},
	},
	{
		ID:              "block-image-tags",
		Code:            "bitg",
		Name:            "Block Image Tags",
		Description:     "Prevents workloads from using container images with the specified tags.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params: []ParamSpec{
			{
				Key:         "tags",
				Label:       "Blocked Tags",
				Type:        ParamTypeStringArray,
				Required:    true,
				Placeholder: "e.g. latest, dev, snapshot",
				Description: "Image tags that are not permitted during the protection window.",
			},
		},
	},
	{
		ID:              "block-replica-scaling",
		Code:            "brs",
		Name:            "Block Replica Scaling",
		Description:     "Prevents changes to replica counts on Deployments and StatefulSets within the target applications.",
		SupportedScopes: []ScopeSupport{ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-storage-changes",
		Code:            "bsc",
		Name:            "Block Storage Changes",
		Description:     "Prevents creation, deletion, or modification of PersistentVolumeClaims and volume definitions on workloads.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-config-secret-resource-changes",
		Code:            "bcsr",
		Name:            "Block ConfigMap and Secret Changes",
		Description:     "Prevents updates or deletion of ConfigMaps and Secrets within the target scope.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
	{
		ID:              "block-workload-config-mount-changes",
		Code:            "bwcm",
		Name:            "Block Workload Config Mount Changes",
		Description:     "Prevents modifications to volume mounts or environment variable sources referencing ConfigMaps or Secrets on workloads.",
		SupportedScopes: []ScopeSupport{ScopeSupportNamespaces, ScopeSupportApplications},
		Params:          nil,
	},
}

var templateIndex = func() map[string]*Template {
	idx := make(map[string]*Template, len(Templates))
	for i := range Templates {
		idx[Templates[i].ID] = &Templates[i]
	}
	return idx
}()

func GetTemplate(id string) (*Template, bool) {
	t, ok := templateIndex[id]
	return t, ok
}

func ValidateParams(template *Template, params map[string]any) error {
	for _, spec := range template.Params {
		if !spec.Required {
			continue
		}
		val, exists := params[spec.Key]
		if !exists || val == nil {
			return fmt.Errorf("param %q is required for template %q", spec.Key, template.ID)
		}
		if spec.Type == ParamTypeStringArray {
			arr, ok := toStringSlice(val)
			if !ok || len(arr) == constants.DefaultInitValue {
				return fmt.Errorf("param %q must be a non-empty string array for template %q", spec.Key, template.ID)
			}
		}
	}
	return nil
}

func toStringSlice(v any) ([]string, bool) {
	switch cast := v.(type) {
	case []string:
		return cast, true
	case []any:
		out := make([]string, constants.DefaultInitValue, len(cast))
		for _, item := range cast {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}
