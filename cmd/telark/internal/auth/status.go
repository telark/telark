package auth

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// The telark-auth role may get TelarkConfig default, whose status.bootstrap holds the pin and every request.
func (o *options) status(ctx context.Context, w io.Writer) error {
	core, objects, err := o.clients()
	if err != nil {
		return err
	}
	if err := served(core, fmt.Sprintf(constants.OldStatusCommand, o.kube.Namespace)); err != nil {
		return err
	}
	configs := objects.Resource(requestsResource.GroupVersion().WithResource(v1alpha1.PluralTelarkConfigs))
	config, err := configs.Namespace(o.kube.Namespace).Get(ctx, telarkconfig.TelarkConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf(constants.ErrReadStatus, err)
	}
	var bootstrap telarkconfig.BootstrapStatus
	raw, _, _ := unstructured.NestedMap(config.Object, constants.KeyStatus, telarkconfig.FieldBootstrap)
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &bootstrap)
	status := Status{Email: bootstrap.Email, PinnedAt: bootstrap.PinnedAt, Pinned: bootstrap.Pinned(), NewestRequest: bootstrap.Newest()}
	if o.format != output.FormatTable {
		return output.Write(w, o.format, status)
	}
	text := constants.MsgStatusUnpinned
	if status.Pinned {
		text = fmt.Sprintf(constants.MsgStatusPinned, status.Email, status.PinnedAt)
	}
	_, err = io.WriteString(w, text+describe(status.Email, status.NewestRequest))
	return err
}

// Requests opened before 0.3.0 name no Kubernetes user.
func describe(pinnedEmail string, request *telarkconfig.BootstrapRequest) string {
	if request == nil {
		return constants.MsgStatusNoRequest
	}
	kind := fmt.Sprintf(constants.MsgKindRecover, pinnedEmail)
	if request.Kind == telarkconfig.BootstrapKindReplace {
		kind = fmt.Sprintf(constants.MsgKindReplace, request.TargetEmail)
	}
	state := strings.TrimSpace(request.State + constants.Space + request.Reason)
	if request.RequestedBy == constants.EmptyString {
		return fmt.Sprintf(constants.MsgStatusRequest, request.ID, kind, state, request.OpenedAt)
	}
	return fmt.Sprintf(constants.MsgStatusRequestBy, request.ID, kind, state, request.OpenedAt, request.RequestedBy)
}
