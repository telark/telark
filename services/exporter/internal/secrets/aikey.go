package secrets

import (
	"context"
	"fmt"
	"os"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/k8sclient"
	k8scorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// The provider key lives in a Secret rather than on the GlobalConfig CR: a CRD
// has no field-level RBAC, so every reader of the config could read the key.
type AIKeyStore struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

func NewAIKeyStore() (*AIKeyStore, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrAIKeyClientUnavailable), err)
	}

	return NewAIKeyStoreWithClient(
		client,
		os.Getenv(constants.AIKeySecretNamespaceEnv),
		os.Getenv(constants.AIKeySecretNameEnv),
	), nil
}

func NewAIKeyStoreWithClient(client kubernetes.Interface, namespace, name string) *AIKeyStore {
	return &AIKeyStore{client: client, namespace: namespace, name: name}
}

// An absent secret or field is "no key configured", not an error — that is the
// state of a fresh install before an admin sets one.
func (s *AIKeyStore) Get(ctx context.Context) (string, error) {
	secret, err := s.fetch(ctx)
	if err != nil || secret == nil {
		return constants.EmptyString, err
	}

	return string(secret.Data[constants.AIKeySecretField]), nil
}

func (s *AIKeyStore) Set(ctx context.Context, key string) error {
	secret, err := s.fetch(ctx)
	if err != nil {
		return err
	}
	if secret == nil {
		return s.create(ctx, key)
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[constants.AIKeySecretField] = []byte(key)

	return s.update(ctx, secret)
}

func (s *AIKeyStore) Clear(ctx context.Context) error {
	secret, err := s.fetch(ctx)
	if err != nil || secret == nil {
		return err
	}

	delete(secret.Data, constants.AIKeySecretField)

	return s.update(ctx, secret)
}

func (s *AIKeyStore) create(ctx context.Context, key string) error {
	secret := &k8scorev1.Secret{
		ObjectMeta: k8smetav1.ObjectMeta{Name: s.name, Namespace: s.namespace},
		Data:       map[string][]byte{constants.AIKeySecretField: []byte(key)},
	}

	if _, err := s.secrets().Create(ctx, secret, k8smetav1.CreateOptions{}); err != nil {
		return fmt.Errorf(string(constants.ErrAIKeyWriteFailed), err)
	}
	return nil
}

func (s *AIKeyStore) update(ctx context.Context, secret *k8scorev1.Secret) error {
	if _, err := s.secrets().Update(ctx, secret, k8smetav1.UpdateOptions{}); err != nil {
		return fmt.Errorf(string(constants.ErrAIKeyWriteFailed), err)
	}
	return nil
}

func (s *AIKeyStore) fetch(ctx context.Context) (*k8scorev1.Secret, error) {
	secret, err := s.secrets().Get(ctx, s.name, k8smetav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrAIKeyReadFailed), err)
	}
	return secret, nil
}

func (s *AIKeyStore) secrets() typedcorev1.SecretInterface {
	return s.client.CoreV1().Secrets(s.namespace)
}
