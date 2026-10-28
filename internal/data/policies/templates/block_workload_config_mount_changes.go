package templates

import (
	"strings"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockMountChanges = "block-workload-config-mount-changes"
	codeBlockMountChanges     = "bwcm"
)

type blockMountChanges struct{}

func (blockMountChanges) TemplateID() string   { return templateBlockMountChanges }
func (blockMountChanges) TemplateCode() string { return codeBlockMountChanges }

func (blockMountChanges) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	return policies.RenderPodSpecRulePolicy(meta, scope, policies.PodSpecRuleSpec{
		TemplateID:   templateBlockMountChanges,
		TemplateCode: codeBlockMountChanges,
		Ops:          opsUpdate,
		Message:      blockMessage(meta, msgBlockConfigMountChng),
		Admit:        mountsUnchanged,
	}), nil
}

// Every container list, not just containers[]: an initContainer envFrom went unnoticed.
var mountsUnchanged = strings.Join([]string{
	same(exprCMVolumesFmt, policies.VarPodSpec, policies.VarOldPodSpec),
	same(exprSecretVolumesFmt, policies.VarPodSpec, policies.VarOldPodSpec),
	same(exprEnvFromFmt, policies.VarContainers, policies.VarOldContainers),
	same(exprEnvRefsFmt, policies.VarContainers, policies.VarOldContainers),
	same(exprVolumeMountsFmt, policies.VarContainers, policies.VarOldContainers),
}, celAnd)

func init() {
	policies.Register(blockMountChanges{})
}
