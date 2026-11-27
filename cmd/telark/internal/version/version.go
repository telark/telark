package version

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/telark/telark/cmd/telark/internal/constants"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/kubernetes"
)

// Set at build time with -ldflags -X (the Makefile's cli target).
var (
	version   = constants.DevVersion
	gitCommit = constants.UnknownValue
	buildDate = constants.UnknownValue
)

func Client() ClientInfo {
	return ClientInfo{Version: version, GitCommit: gitCommit, BuildDate: buildDate, GoVersion: runtime.Version()}
}

func Server(ctx context.Context, client kubernetes.Interface, namespace string) (*ServerInfo, error) {
	list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: constants.TelarkSelector})
	if err != nil {
		return nil, fmt.Errorf(constants.ErrListDeployments, namespace, err)
	}
	if len(list.Items) == constants.DefaultInitValue {
		return nil, fmt.Errorf(constants.ErrNoDeployments, namespace)
	}
	return Summarize(namespace, list.Items), nil
}

// Telark has no version route, so the server version is the chart's version label on its Deployments.
func Summarize(namespace string, deployments []appsv1.Deployment) *ServerInfo {
	info := &ServerInfo{Namespace: namespace, Services: make(map[string]string, len(deployments))}
	versions := make([]string, constants.DefaultInitValue, len(deployments))
	for i := range deployments {
		deployment := &deployments[i]
		versions = append(versions, deployment.Labels[constants.LabelVersion])
		info.Services[serviceName(deployment)] = imageTag(deployment.Spec.Template.Spec.Containers[constants.DefaultInitValue].Image)
	}
	slices.Sort(versions)
	info.Version = strings.Join(slices.Compact(versions), constants.ListSeparator)
	return info
}

func serviceName(deployment *appsv1.Deployment) string {
	return strings.TrimSuffix(deployment.Labels[constants.LabelComponent], constants.ServiceSuffix)
}

func imageTag(image string) string {
	name, _, _ := strings.Cut(image, constants.ImageDigestSeparator)
	tag := strings.LastIndex(name, constants.ImageTagSeparator)
	if tag > strings.LastIndex(name, constants.ImagePathSeparator) {
		return name[tag+constants.DefaultIncrementValue:]
	}
	return image
}

// A patch gap is normal, so only a major or minor gap warns.
// A half-finished upgrade reports several server versions, and one gap among them is enough.
func Skewed(client, server string) bool {
	return slices.ContainsFunc(strings.Split(server, constants.ListSeparator), func(label string) bool {
		clientVersion, clientErr := utilversion.ParseGeneric(client)
		serverVersion, serverErr := utilversion.ParseGeneric(label)
		if clientErr != nil || serverErr != nil {
			return false
		}
		return clientVersion.Major() != serverVersion.Major() || clientVersion.Minor() != serverVersion.Minor()
	})
}
