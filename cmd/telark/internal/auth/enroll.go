package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/constants"
	"github.com/telark/telark/cmd/telark/internal/output"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"golang.org/x/crypto/nacl/box"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

var requestsResource = schema.GroupVersionResource{Group: base.Group, Version: base.V1Alpha1, Resource: v1alpha1.PluralAuthRequests}

// The admission policy refuses a requestedBy that isn't the caller's own username, so the API server says who that is.
func (o *options) enroll(ctx context.Context, w io.Writer) error {
	core, objects, err := o.clients()
	if err != nil {
		return err
	}
	if err := served(core, fmt.Sprintf(constants.OldEnrollCommand, o.kube.Namespace, o.email)); err != nil {
		return err
	}
	review, err := core.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf(constants.ErrWhoAmI, err)
	}
	username := review.Status.UserInfo.Username
	// crypto/rand never fails since Go 1.24, so neither does the key.
	public, private, _ := box.GenerateKey(rand.Reader)
	requests := objects.Resource(requestsResource).Namespace(o.kube.Namespace)
	created, err := requests.Create(ctx, newRequest(o.email, username, public), metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf(constants.ErrCreateRequest, err)
	}
	_, _ = fmt.Fprintf(w, constants.MsgRequestCreated, created.GetName(), username)
	e := &enrollment{out: w, printer: output.NewPrinter(w, output.UseColor(w)), name: created.GetName(), public: public, private: private}
	_, err = watchtools.UntilWithSync(ctx, follow(requests, created.GetName()), &unstructured.Unstructured{}, nil, e.step)
	return err
}

func newRequest(email, username string, key *[constants.SealedKeyBytes]byte) *unstructured.Unstructured {
	request := &unstructured.Unstructured{Object: map[string]any{
		constants.KeySpec: map[string]any{
			authrequest.FieldEmail:        email,
			authrequest.FieldRequestedBy:  username,
			authrequest.FieldRecipientKey: base64.StdEncoding.EncodeToString(key[:]),
		},
	}}
	request.SetAPIVersion(requestsResource.GroupVersion().String())
	request.SetKind(v1alpha1.KindAuthRequest)
	request.SetGenerateName(constants.AuthRequestPrefix)
	return request
}

// The informer behind UntilWithSync lists and watches again after a dropped connection, so a long wait survives it.
func follow(requests dynamic.ResourceInterface, name string) *cache.ListWatch {
	selector := fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()
	return &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
			opts.FieldSelector = selector
			return requests.List(ctx, opts)
		},
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.FieldSelector = selector
			return requests.Watch(ctx, opts)
		},
	}
}

// WaitingForApproval ends the command: the object rests there, and a later run continues the same request.
func (e *enrollment) step(event watch.Event) (bool, error) {
	if event.Type == watch.Deleted {
		return false, fmt.Errorf(constants.ErrRequestDeleted, e.name)
	}
	status := statusOf(event.Object)
	if status.Phase == e.phase {
		return false, nil
	}
	e.phase = status.Phase
	switch status.Phase {
	case authrequest.PhasePending:
		return false, nil
	case authrequest.PhaseRecovering:
		e.line(status)
		return false, nil
	case authrequest.PhaseWaitingForApproval:
		e.printer.Warn(fmt.Sprintf(constants.MsgRequestStep, status.Phase, status.Reason))
		e.urls(constants.MsgApprovalURL, status.ApprovalURL, constants.MsgCompletesAt, status.CompletesAt)
		return true, nil
	case authrequest.PhaseLinkReady:
		return false, e.link(status)
	case authrequest.PhaseCompleted:
		e.printer.Pass(fmt.Sprintf(constants.MsgRequestStep, status.Phase, status.Reason))
		return true, nil
	default:
		return false, fmt.Errorf(constants.ErrRequestEnded, e.name, status.Phase, status.Reason)
	}
}

func (e *enrollment) link(status authrequest.Status) error {
	sealed, _ := base64.StdEncoding.DecodeString(status.SealedLink)
	link, opened := box.OpenAnonymous(nil, sealed, e.public, e.private)
	if !opened {
		return fmt.Errorf(constants.ErrSealedLink, e.name)
	}
	e.line(status)
	e.urls(constants.MsgLink, string(link), constants.MsgLinkExpiresAt, status.LinkExpiresAt)
	_, _ = fmt.Fprint(e.out, constants.MsgWaitingPasskey)
	return nil
}

func (e *enrollment) line(status authrequest.Status) {
	_, _ = fmt.Fprintf(e.out, constants.MsgRequestStep+constants.NewLine, status.Phase, status.Reason)
}

// Without app.auth.passkey.origin, auth starts its URLs with a placeholder the reader replaces.
func (e *enrollment) urls(urlFormat, url, timeFormat, at string) {
	_, _ = fmt.Fprintf(e.out, urlFormat, url)
	_, _ = fmt.Fprintf(e.out, timeFormat, at)
	if strings.HasPrefix(url, constants.DashboardPlaceholder) {
		_, _ = fmt.Fprint(e.out, constants.MsgPlaceholder)
	}
}

// The informer only hands over the objects the watch decoded, all of them unstructured.
func statusOf(object runtime.Object) authrequest.Status {
	var status authrequest.Status
	if item, ok := object.(*unstructured.Unstructured); ok {
		raw, _, _ := unstructured.NestedMap(item.Object, constants.KeyStatus)
		_ = runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &status)
	}
	return status
}
