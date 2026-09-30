package oidctrust

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	globalshared "github.com/telark/telark/internal/data/shared"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/config"
	"github.com/telark/telark/services/exporter/internal/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var secretGVR = schema.GroupVersionResource{Version: constants.SecretVersion, Resource: constants.SecretResource}

// The Google JWK set lives in the OIDC trust Secret, never in the TelarkConfig CR:
// auth reads it from that Secret mounted as a file.
func ReadJWK() (string, error) {
	name := config.OIDCTrustSecretName()
	client, err := secrets()
	if err != nil {
		return constants.EmptyString, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.OIDCTrustTimeout)
	defer cancel()

	secret, err := client.Get(ctx, name, k8smetav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return constants.EmptyString, nil
	}
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrOIDCTrustSecretRead), name, err)
	}

	encoded, _, _ := unstructured.NestedString(secret.Object, constants.SecretDataField, telarkconfig.OIDCSecretKey)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrOIDCTrustSecretDecode), name, telarkconfig.OIDCSecretKey, err)
	}
	return string(decoded), nil
}

// The chart renders the Secret, so this only patches its key: the exporter holds
// no create right on Secrets.
func WriteJWK(value string) error {
	name := config.OIDCTrustSecretName()
	client, err := secrets()
	if err != nil {
		return err
	}
	patch, err := json.Marshal(map[string]any{
		constants.SecretDataField: map[string]any{
			telarkconfig.OIDCSecretKey: base64.StdEncoding.EncodeToString([]byte(value)),
		},
	})
	if err != nil {
		return fmt.Errorf(string(constants.ErrOIDCTrustSecretWrite), name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.OIDCTrustTimeout)
	defer cancel()
	if _, err := client.Patch(ctx, name, types.MergePatchType, patch, k8smetav1.PatchOptions{}); err != nil {
		return fmt.Errorf(string(constants.ErrOIDCTrustSecretWrite), name, err)
	}
	return nil
}

func secrets() (dynamic.ResourceInterface, error) {
	client, err := k8sclient.InitDynamicClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCTrustSecretClient), err)
	}
	return client.Resource(secretGVR).Namespace(globalshared.BaseNamespace), nil
}
