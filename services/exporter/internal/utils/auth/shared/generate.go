package shared

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var K8sNameUnsafe = regexp.MustCompile(`[^a-z0-9-]+`)

func SanitizeK8sName(input string) string {
	s := strings.ToLower(input)
	s = K8sNameUnsafe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == constants.EmptyString {
		s = "default"
	}
	return s
}

func GenerateCRDName(userID string, resourceType string, findResourcesFunc func(string) ([]unstructured.Unstructured, error)) (string, error) {
	username, err := userutils.GetUsernameFromUser(userID)
	if err != nil {
		return constants.EmptyString, err
	}

	resources, err := findResourcesFunc(userID)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrFailedToListResources), resourceType, err)
	}

	return fmt.Sprintf("%s-%s-%d", SanitizeK8sName(username), resourceType, len(resources)+constants.DefaultIncrementValue), nil
}
